package agentconnector

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jdhxyy/agent-connector/mqtt"
	"github.com/jdhxyy/agent-connector/protocol"
	"github.com/jdhxyy/agent-connector/router"
	"github.com/jdhxyy/agent-connector/websocket"
)

type connector struct {
	config        *Config
	wsClient      *websocket.Client
	mqttClient    *mqtt.Client
	router        *router.Router
	converter     *protocol.Converter
	
	status        ConnectorStatus
	statusMu      sync.RWMutex
	
	msgHandlers   []MessageHandler
	errHandlers   []ErrorHandler
	statusHandlers []StatusHandler
	
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

func NewConnector(config *Config) (Connector, error) {
	if config == nil {
		return nil, ErrInvalidConfig
	}

	if config.AgentID == "" {
		return nil, fmt.Errorf("%w: agent ID is required", ErrInvalidConfig)
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &connector{
		config:         config,
		converter:      protocol.NewConverter(),
		router:         router.NewRouter(),
		msgHandlers:    make([]MessageHandler, 0),
		errHandlers:    make([]ErrorHandler, 0),
		statusHandlers: make([]StatusHandler, 0),
		ctx:            ctx,
		cancel:         cancel,
	}

	wsConfig := &websocket.Config{
		BaseURL:           config.WebSocket.BaseURL,
		Token:             config.WebSocket.Token,
		SessionID:         config.WebSocket.SessionID,
		HandshakeTimeout:  config.WebSocket.HandshakeTimeout,
		PingInterval:      config.WebSocket.PingInterval,
		ReconnectMaxRetry: config.WebSocket.ReconnectMaxRetry,
		ReconnectDelay:    config.WebSocket.ReconnectDelay,
	}
	c.wsClient = websocket.NewClient(wsConfig)

	mqttConfig := &mqtt.Config{
		BrokerURL:            config.MQTT.BrokerURL,
		ClientID:             config.MQTT.ClientID,
		Username:             config.MQTT.Username,
		Password:             config.MQTT.Password,
		KeepAlive:            config.MQTT.KeepAlive,
		ConnectTimeout:       config.MQTT.ConnectTimeout,
		CleanSession:         config.MQTT.CleanSession,
		AutoReconnect:        config.MQTT.AutoReconnect,
		MaxReconnectInterval: config.MQTT.MaxReconnectInterval,
	}
	c.mqttClient = mqtt.NewClient(mqttConfig)

	c.setupRouter()

	return c, nil
}

func (c *connector) setupRouter() {
	c.router.SetWebSocketHandler(func(msg protocol.Message) error {
		return c.handleWebSocketMessage(msg)
	})
	c.router.SetMQTTHandler(func(msg protocol.Message) error {
		return c.handleMQTTMessage(msg)
	})
}

func (c *connector) Start(ctx context.Context) error {
	if err := c.wsClient.Connect(ctx); err != nil {
		return fmt.Errorf("websocket connect: %w", err)
	}

	if err := c.mqttClient.Connect(ctx); err != nil {
		c.wsClient.Disconnect()
		return fmt.Errorf("mqtt connect: %w", err)
	}

	c.updateStatus(func(s *ConnectorStatus) {
		s.WebSocketStatus = StatusConnected
		s.MQTTStatus = StatusConnected
		s.IsRunning = true
		s.StartTime = time.Now()
	})

	c.wg.Add(1)
	go c.messageLoop()

	return nil
}

func (c *connector) Stop() error {
	c.cancel()

	c.wsClient.Disconnect()
	c.mqttClient.Disconnect(5 * time.Second)

	c.wg.Wait()

	c.updateStatus(func(s *ConnectorStatus) {
		s.WebSocketStatus = StatusDisconnected
		s.MQTTStatus = StatusDisconnected
		s.IsRunning = false
	})

	return nil
}

func (c *connector) Status() ConnectorStatus {
	c.statusMu.RLock()
	defer c.statusMu.RUnlock()
	return c.status
}

func (c *connector) SendToAgent(agentID string, msg protocol.Message) error {
	topic := fmt.Sprintf("agent/chat/%s/%s", agentID, msg.GetType())
	
	payload, err := msg.GetPayload(), nil
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	return c.mqttClient.Publish(topic, c.config.Router.DefaultQoS, false, payload)
}

func (c *connector) Broadcast(msg protocol.Message) error {
	topic := fmt.Sprintf("agent/chat/broadcast/%s", msg.GetType())
	
	payload, err := msg.GetPayload(), nil
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	return c.mqttClient.Publish(topic, c.config.Router.DefaultQoS, false, payload)
}

func (c *connector) SubscribeAgent(agentID string) error {
	topics := map[string]byte{
		fmt.Sprintf("agent/chat/%s/text", agentID):    c.config.Router.DefaultQoS,
		fmt.Sprintf("agent/chat/%s/command", agentID): c.config.Router.DefaultQoS,
		fmt.Sprintf("agent/chat/%s/status", agentID):  c.config.Router.DefaultQoS,
	}

	return c.mqttClient.SubscribeMultiple(topics, func(msg protocol.Message) error {
		return c.handleIncomingMessage(msg)
	})
}

func (c *connector) UnsubscribeAgent(agentID string) error {
	topics := []string{
		fmt.Sprintf("agent/chat/%s/text", agentID),
		fmt.Sprintf("agent/chat/%s/command", agentID),
		fmt.Sprintf("agent/chat/%s/status", agentID),
	}

	return c.mqttClient.Unsubscribe(topics...)
}

func (c *connector) OnMessage(handler MessageHandler) {
	c.msgHandlers = append(c.msgHandlers, handler)
}

func (c *connector) OnError(handler ErrorHandler) {
	c.errHandlers = append(c.errHandlers, handler)
}

func (c *connector) OnStatusChange(handler StatusHandler) {
	c.statusHandlers = append(c.statusHandlers, handler)
}

func (c *connector) messageLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.config.WebSocket.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if err := c.wsClient.SendPing(); err != nil {
				c.notifyError(err, ErrorContext{
					Source:    "websocket",
					Operation: "ping",
				})
			}
		default:
			msg, err := c.wsClient.ReceiveMessage(100 * time.Millisecond)
			if err != nil {
				continue
			}

			genericMsg, err := c.converter.PicoToGeneric(msg, c.config.AgentID)
			if err != nil {
				c.notifyError(err, ErrorContext{
					Source:    "websocket",
					Operation: "convert",
				})
				continue
			}

			if err := c.handleIncomingMessage(genericMsg); err != nil {
				c.notifyError(err, ErrorContext{
					Source:    "connector",
					Operation: "handle",
				})
			}
		}
	}
}

func (c *connector) handleIncomingMessage(msg protocol.Message) error {
	for _, handler := range c.msgHandlers {
		if err := handler(msg); err != nil {
			return err
		}
	}
	return nil
}

func (c *connector) handleWebSocketMessage(msg protocol.Message) error {
	return c.SendToAgent(msg.GetTarget(), msg)
}

func (c *connector) handleMQTTMessage(msg protocol.Message) error {
	return nil
}

func (c *connector) updateStatus(fn func(*ConnectorStatus)) {
	c.statusMu.Lock()
	oldStatus := c.status
	fn(&c.status)
	newStatus := c.status
	c.statusMu.Unlock()

	for _, handler := range c.statusHandlers {
		handler(oldStatus.WebSocketStatus, newStatus.WebSocketStatus)
	}
}

func (c *connector) notifyError(err error, ctx ErrorContext) {
	ctx.Timestamp = time.Now().Unix()
	for _, handler := range c.errHandlers {
		handler(err, ctx)
	}
}
