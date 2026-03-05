package agentconnector

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jdhxyy/agent-connector/mqtt"
	"github.com/jdhxyy/agent-connector/protocol"
	"github.com/jdhxyy/agent-connector/websocket"
)

// connector 是 Connector 接口的具体实现
type connector struct {
	config     *Config
	wsClient   *websocket.Client
	mqttClient *mqtt.Client
	converter  *protocol.Converter

	status   ConnectorStatus
	statusMu sync.RWMutex

	msgHandlers    []MessageHandler
	errHandlers    []ErrorHandler
	statusHandlers []StatusHandler

	subscribedAgents map[string]bool
	subscribedGroups map[string]bool
	subscribedMu     sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewConnector 创建并初始化一个新的 Connector 实例
func NewConnector(config *Config) (Connector, error) {
	if config == nil {
		log.Printf("[ERROR] [NewConnector] Config cannot be nil")
		return nil, ErrInvalidConfig
	}

	log.Printf("[INFO] [NewConnector] Creating Connector")

	ctx, cancel := context.WithCancel(context.Background())

	c := &connector{
		config:           config,
		converter:        protocol.NewConverter(),
		msgHandlers:      make([]MessageHandler, 0),
		errHandlers:      make([]ErrorHandler, 0),
		statusHandlers:   make([]StatusHandler, 0),
		subscribedAgents: make(map[string]bool),
		subscribedGroups: make(map[string]bool),
		ctx:              ctx,
		cancel:           cancel,
	}

	wsConfig := &websocket.Config{
		BaseURL:          config.WebSocket.BaseURL,
		Token:            config.WebSocket.Token,
		SessionID:        config.WebSocket.SessionID,
		HandshakeTimeout: config.WebSocket.HandshakeTimeout,
		PingInterval:     config.WebSocket.PingInterval,
		ConnectTimeout:   config.WebSocket.ConnectTimeout,
	}
	c.wsClient = websocket.NewClient(wsConfig)
	log.Printf("[INFO] [NewConnector] WebSocket client created, URL=%s", config.WebSocket.BaseURL)

	mqttConfig := &mqtt.Config{
		BrokerURL:      config.MQTT.BrokerURL,
		ClientID:       config.MQTT.ClientID,
		Username:       config.MQTT.Username,
		Password:       config.MQTT.Password,
		KeepAlive:      config.MQTT.KeepAlive,
		ConnectTimeout: config.MQTT.ConnectTimeout,
		CleanSession:   config.MQTT.CleanSession,
	}
	c.mqttClient = mqtt.NewClient(mqttConfig)
	log.Printf("[INFO] [NewConnector] MQTT client created, Broker=%s", config.MQTT.BrokerURL)

	log.Printf("[INFO] [NewConnector] Connector created successfully")

	return c, nil
}

// Start 启动连接器，建立 WebSocket 和 MQTT 连接
func (c *connector) Start(ctx context.Context) error {
	log.Printf("[INFO] [Start] Starting Connector")

	log.Printf("[INFO] [Start] Connecting WebSocket, URL=%s", c.config.WebSocket.BaseURL)
	if err := c.wsClient.Connect(ctx); err != nil {
		log.Printf("[ERROR] [Start] WebSocket connection failed: %v", err)
		return fmt.Errorf("websocket connect: %w", err)
	}
	log.Printf("[INFO] [Start] WebSocket connected")

	log.Printf("[INFO] [Start] Connecting MQTT, Broker=%s", c.config.MQTT.BrokerURL)
	if err := c.mqttClient.Connect(ctx); err != nil {
		log.Printf("[ERROR] [Start] MQTT connection failed: %v", err)
		c.wsClient.Disconnect()
		return fmt.Errorf("mqtt connect: %w", err)
	}
	log.Printf("[INFO] [Start] MQTT connected")

	c.updateStatus(func(s *ConnectorStatus) {
		s.WebSocketStatus = StatusConnected
		s.MQTTStatus = StatusConnected
		s.IsRunning = true
		s.StartTime = time.Now()
	})

	c.wg.Add(1)
	go c.messageLoop()

	// 启动连接监控和自动重连
	c.wg.Add(1)
	go c.connectionMonitor()

	log.Printf("[INFO] [Start] Connector started successfully")

	return nil
}

// Stop 停止连接器，断开所有连接
func (c *connector) Stop() error {
	log.Printf("[INFO] [Stop] Stopping Connector")

	c.cancel()
	log.Printf("[DEBUG] [Stop] Context cancelled")

	log.Printf("[INFO] [Stop] Disconnecting WebSocket")
	c.wsClient.Disconnect()
	log.Printf("[INFO] [Stop] WebSocket disconnected")

	log.Printf("[INFO] [Stop] Disconnecting MQTT")
	c.mqttClient.Disconnect(5 * time.Second)
	log.Printf("[INFO] [Stop] MQTT disconnected")

	log.Printf("[DEBUG] [Stop] Waiting for all goroutines to exit")
	c.wg.Wait()
	log.Printf("[DEBUG] [Stop] All goroutines exited")

	c.updateStatus(func(s *ConnectorStatus) {
		s.WebSocketStatus = StatusDisconnected
		s.MQTTStatus = StatusDisconnected
		s.IsRunning = false
	})

	log.Printf("[INFO] [Stop] Connector stopped")

	return nil
}

// Status 获取当前连接器的状态
func (c *connector) Status() ConnectorStatus {
	c.statusMu.RLock()
	defer c.statusMu.RUnlock()
	return c.status
}

// SubscribeExternalAgents 订阅外部 Agent 消息
func (c *connector) SubscribeExternalAgents(agentIDs []string) error {
	log.Printf("[INFO] [SubscribeExternalAgents] Subscribing external Agents: %v", agentIDs)

	for _, agentID := range agentIDs {
		topic := protocol.BuildAgentTopic(agentID)
		if err := c.mqttClient.Subscribe(topic, c.config.Router.DefaultQoS, c.mqttMessageHandler); err != nil {
			return fmt.Errorf("subscribe to agent %s failed: %w", agentID, err)
		}

		c.subscribedMu.Lock()
		c.subscribedAgents[agentID] = true
		c.subscribedMu.Unlock()

		log.Printf("[INFO] [SubscribeExternalAgents] Subscribed external Agent: %s (topic: %s)", agentID, topic)
	}

	return nil
}

// SubscribeExternalGroups 订阅外部群组消息
func (c *connector) SubscribeExternalGroups(groupIDs []string) error {
	log.Printf("[INFO] [SubscribeExternalGroups] Subscribing external groups: %v", groupIDs)

	for _, groupID := range groupIDs {
		topic := protocol.BuildGroupTopic(groupID)
		if err := c.mqttClient.Subscribe(topic, c.config.Router.DefaultQoS, c.mqttMessageHandler); err != nil {
			return fmt.Errorf("subscribe to group %s failed: %w", groupID, err)
		}

		c.subscribedMu.Lock()
		c.subscribedGroups[groupID] = true
		c.subscribedMu.Unlock()

		log.Printf("[INFO] [SubscribeExternalGroups] Subscribed external group: %s (topic: %s)", groupID, topic)
	}

	return nil
}

// UnsubscribeExternalAgents 取消订阅外部 Agent
func (c *connector) UnsubscribeExternalAgents(agentIDs []string) error {
	log.Printf("[INFO] [UnsubscribeExternalAgents] Unsubscribing external Agents: %v", agentIDs)

	for _, agentID := range agentIDs {
		topic := protocol.BuildAgentTopic(agentID)
		if err := c.mqttClient.Unsubscribe(topic); err != nil {
			return fmt.Errorf("unsubscribe from agent %s failed: %w", agentID, err)
		}

		c.subscribedMu.Lock()
		delete(c.subscribedAgents, agentID)
		c.subscribedMu.Unlock()

		log.Printf("[INFO] [UnsubscribeExternalAgents] Unsubscribed external Agent: %s", agentID)
	}

	return nil
}

// UnsubscribeExternalGroups 取消订阅外部群组
func (c *connector) UnsubscribeExternalGroups(groupIDs []string) error {
	log.Printf("[INFO] [UnsubscribeExternalGroups] Unsubscribing external groups: %v", groupIDs)

	for _, groupID := range groupIDs {
		topic := protocol.BuildGroupTopic(groupID)
		if err := c.mqttClient.Unsubscribe(topic); err != nil {
			return fmt.Errorf("unsubscribe from group %s failed: %w", groupID, err)
		}

		c.subscribedMu.Lock()
		delete(c.subscribedGroups, groupID)
		c.subscribedMu.Unlock()

		log.Printf("[INFO] [UnsubscribeExternalGroups] Unsubscribed external group: %s", groupID)
	}

	return nil
}

// OnMessage 注册消息处理函数
func (c *connector) OnMessage(handler MessageHandler) {
	c.msgHandlers = append(c.msgHandlers, handler)
	log.Printf("[DEBUG] [OnMessage] Message handler registered, current count: %d", len(c.msgHandlers))
}

// OnError 注册错误处理函数
func (c *connector) OnError(handler ErrorHandler) {
	c.errHandlers = append(c.errHandlers, handler)
	log.Printf("[DEBUG] [OnError] Error handler registered, current count: %d", len(c.errHandlers))
}

// OnStatusChange 注册状态变更处理函数
func (c *connector) OnStatusChange(handler StatusHandler) {
	c.statusHandlers = append(c.statusHandlers, handler)
	log.Printf("[DEBUG] [OnStatusChange] Status handler registered, current count: %d", len(c.statusHandlers))
}

// mqttMessageHandler 统一的 MQTT 消息处理函数
func (c *connector) mqttMessageHandler(topic string, data []byte) {
	log.Printf("[DEBUG] [mqttMessageHandler] Received MQTT message, Topic=%s, Content=%s", topic, string(data))

	picoMsg, err := c.converter.MQTTToPico(topic, data)
	if err != nil {
		log.Printf("[ERROR] [mqttMessageHandler] Failed to convert MQTT message: %v", err)
		c.notifyError(err, ErrorContext{
			Source:    "mqtt",
			Operation: "convert",
		})
		return
	}

	if err := c.wsClient.SendPicoMessage(picoMsg); err != nil {
		log.Printf("[ERROR] [mqttMessageHandler] Failed to forward to WebSocket: %v", err)
		c.notifyError(err, ErrorContext{
			Source:    "websocket",
			Operation: "send",
		})
		return
	}

	log.Printf("[DEBUG] [mqttMessageHandler] MQTT message forwarded to WebSocket, SessionID=%s", picoMsg.SessionID)
}

// messageLoop 消息处理循环
func (c *connector) messageLoop() {
	defer c.wg.Done()

	log.Printf("[INFO] [messageLoop] Message loop started")

	ticker := time.NewTicker(c.config.WebSocket.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			log.Printf("[INFO] [messageLoop] Received exit signal, message loop ended")
			return

		case <-ticker.C:
			log.Printf("[DEBUG] [messageLoop] Sending WebSocket ping")
			if err := c.wsClient.SendPing(); err != nil {
				log.Printf("[ERROR] [messageLoop] Ping failed: %v", err)
				c.notifyError(err, ErrorContext{
					Source:    "websocket",
					Operation: "ping",
				})
			}

		default:
			picoMsg, err := c.wsClient.ReceiveMessage(100 * time.Millisecond)
			if err != nil {
				continue
			}

			log.Printf("[DEBUG] [messageLoop] Received WebSocket message, Type=%s, SessionID=%s", picoMsg.Type, picoMsg.SessionID)

			// 只转发 message.send 类型的消息到 MQTT，过滤其他所有类型
			if picoMsg.Type != protocol.TypeMessageSend && picoMsg.Type != protocol.TypeMessageCreate {
				log.Printf("[DEBUG] [messageLoop] Received non-message.send message (Type=%s), not forwarding to MQTT", picoMsg.Type)
				continue
			}

			if err := c.forwardWebSocketToMQTT(picoMsg); err != nil {
				log.Printf("[ERROR] [messageLoop] Failed to forward to MQTT: %v", err)
				c.notifyError(err, ErrorContext{
					Source:    "mqtt",
					Operation: "publish",
				})
			}
		}
	}
}

// forwardWebSocketToMQTT 将 WebSocket 消息转发到 MQTT
func (c *connector) forwardWebSocketToMQTT(picoMsg protocol.PicoMessage) error {
	topic, payload, err := c.converter.PicoToMQTT(picoMsg)
	if err != nil {
		return fmt.Errorf("convert pico to mqtt: %w", err)
	}

	log.Printf("[DEBUG] [forwardWebSocketToMQTT] Forwarding to MQTT, Topic=%s, Content=%s", topic, string(payload))

	if err := c.mqttClient.Publish(topic, c.config.Router.DefaultQoS, false, payload); err != nil {
		return fmt.Errorf("publish to mqtt: %w", err)
	}

	log.Printf("[DEBUG] [forwardWebSocketToMQTT] Forwarded successfully, Topic=%s", topic)
	return nil
}

// updateStatus 更新连接器状态
func (c *connector) updateStatus(fn func(*ConnectorStatus)) {
	c.statusMu.Lock()
	oldStatus := c.status
	fn(&c.status)
	newStatus := c.status
	c.statusMu.Unlock()

	log.Printf("[DEBUG] [updateStatus] Status changed: WS=%s->%s, MQTT=%s->%s, Running=%v->%v",
		oldStatus.WebSocketStatus, newStatus.WebSocketStatus,
		oldStatus.MQTTStatus, newStatus.MQTTStatus,
		oldStatus.IsRunning, newStatus.IsRunning)

	for _, handler := range c.statusHandlers {
		handler(oldStatus.WebSocketStatus, newStatus.WebSocketStatus)
	}
}

// connectionMonitor 连接监控和自动重连
func (c *connector) connectionMonitor() {
	defer c.wg.Done()

	log.Printf("[INFO] [connectionMonitor] Connection monitor started")

	checkInterval := 5 * time.Second
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			log.Printf("[INFO] [connectionMonitor] Received exit signal, monitor stopped")
			return

		case <-ticker.C:
			// 检查 WebSocket 连接状态
			if !c.wsClient.IsConnected() {
				log.Printf("[WARN] [connectionMonitor] WebSocket disconnected, attempting to reconnect...")
				c.updateStatus(func(s *ConnectorStatus) {
					s.WebSocketStatus = StatusDisconnected
				})

				if err := c.reconnectWebSocket(); err != nil {
					log.Printf("[ERROR] [connectionMonitor] WebSocket reconnection failed: %v", err)
				} else {
					log.Printf("[INFO] [connectionMonitor] WebSocket reconnected successfully")
					c.updateStatus(func(s *ConnectorStatus) {
						s.WebSocketStatus = StatusConnected
					})
				}
			}

			// 检查 MQTT 连接状态（仅更新状态，不重连，因为 autopaho 会自动重连）
			if !c.mqttClient.IsConnected() {
				c.updateStatus(func(s *ConnectorStatus) {
					s.MQTTStatus = StatusDisconnected
				})
			} else {
				c.updateStatus(func(s *ConnectorStatus) {
					s.MQTTStatus = StatusConnected
				})
			}
		}
	}
}

// reconnectWebSocket 重新连接 WebSocket
func (c *connector) reconnectWebSocket() error {
	// 先清理旧连接
	c.wsClient.Disconnect()

	attempt := 1
	for {
		select {
		case <-c.ctx.Done():
			return fmt.Errorf("context cancelled")
		default:
		}

		log.Printf("[INFO] [reconnectWebSocket] Reconnection attempt %d", attempt)

		ctx, cancel := context.WithTimeout(c.ctx, c.config.WebSocket.ConnectTimeout)
		err := c.wsClient.Connect(ctx)
		cancel()

		if err == nil {
			return nil
		}

		log.Printf("[WARN] [reconnectWebSocket] Attempt %d failed: %v", attempt, err)
		log.Printf("[INFO] [reconnectWebSocket] Waiting 5s before next attempt")

		select {
		case <-c.ctx.Done():
			return fmt.Errorf("context cancelled")
		case <-time.After(5 * time.Second):
		}

		attempt++
	}
}

// reconnectMQTT 重新连接 MQTT
func (c *connector) reconnectMQTT() error {
	attempt := 1
	for {
		select {
		case <-c.ctx.Done():
			return fmt.Errorf("context cancelled")
		default:
		}

		log.Printf("[INFO] [reconnectMQTT] Reconnection attempt %d", attempt)

		ctx, cancel := context.WithTimeout(c.ctx, c.config.MQTT.ConnectTimeout)
		err := c.mqttClient.Connect(ctx)
		cancel()

		if err == nil {
			// 重新订阅之前的主题
			if err := c.resubscribeTopics(); err != nil {
				log.Printf("[WARN] [reconnectMQTT] Failed to resubscribe topics: %v", err)
			}
			return nil
		}

		log.Printf("[WARN] [reconnectMQTT] Attempt %d failed: %v", attempt, err)
		log.Printf("[INFO] [reconnectMQTT] Waiting 5s before next attempt")

		select {
		case <-c.ctx.Done():
			return fmt.Errorf("context cancelled")
		case <-time.After(5 * time.Second):
		}

		attempt++
	}
}

// resubscribeTopics 重新订阅所有主题
func (c *connector) resubscribeTopics() error {
	c.subscribedMu.RLock()
	agents := make([]string, 0, len(c.subscribedAgents))
	for agentID := range c.subscribedAgents {
		agents = append(agents, agentID)
	}
	groups := make([]string, 0, len(c.subscribedGroups))
	for groupID := range c.subscribedGroups {
		groups = append(groups, groupID)
	}
	c.subscribedMu.RUnlock()

	if len(agents) > 0 {
		if err := c.SubscribeExternalAgents(agents); err != nil {
			return fmt.Errorf("resubscribe agents: %w", err)
		}
	}

	if len(groups) > 0 {
		if err := c.SubscribeExternalGroups(groups); err != nil {
			return fmt.Errorf("resubscribe groups: %w", err)
		}
	}

	return nil
}

// notifyError 通知所有错误处理函数
func (c *connector) notifyError(err error, ctx ErrorContext) {
	ctx.Timestamp = time.Now().Unix()
	log.Printf("[ERROR] [notifyError] Error occurred: Source=%s, Operation=%s, Error=%v", ctx.Source, ctx.Operation, err)

	for _, handler := range c.errHandlers {
		handler(err, ctx)
	}
}
