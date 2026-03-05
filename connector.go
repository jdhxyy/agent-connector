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

	status        ConnectorStatus
	statusMu      sync.RWMutex

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
		log.Printf("[ERROR] [NewConnector] 配置不能为空")
		return nil, ErrInvalidConfig
	}

	log.Printf("[INFO] [NewConnector] 正在创建 Connector")

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
		BaseURL:           config.WebSocket.BaseURL,
		Token:             config.WebSocket.Token,
		SessionID:         config.WebSocket.SessionID,
		HandshakeTimeout:  config.WebSocket.HandshakeTimeout,
		PingInterval:      config.WebSocket.PingInterval,
		ReconnectMaxRetry: config.WebSocket.ReconnectMaxRetry,
		ReconnectDelay:    config.WebSocket.ReconnectDelay,
	}
	c.wsClient = websocket.NewClient(wsConfig)
	log.Printf("[INFO] [NewConnector] WebSocket 客户端已创建, URL=%s", config.WebSocket.BaseURL)

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
	log.Printf("[INFO] [NewConnector] MQTT 客户端已创建, Broker=%s", config.MQTT.BrokerURL)

	log.Printf("[INFO] [NewConnector] Connector 创建成功")

	return c, nil
}

// Start 启动连接器，建立 WebSocket 和 MQTT 连接
func (c *connector) Start(ctx context.Context) error {
	log.Printf("[INFO] [Start] 正在启动 Connector")

	log.Printf("[INFO] [Start] 正在连接 WebSocket, URL=%s", c.config.WebSocket.BaseURL)
	if err := c.wsClient.Connect(ctx); err != nil {
		log.Printf("[ERROR] [Start] WebSocket 连接失败: %v", err)
		return fmt.Errorf("websocket connect: %w", err)
	}
	log.Printf("[INFO] [Start] WebSocket 连接成功")

	log.Printf("[INFO] [Start] 正在连接 MQTT, Broker=%s", c.config.MQTT.BrokerURL)
	if err := c.mqttClient.Connect(ctx); err != nil {
		log.Printf("[ERROR] [Start] MQTT 连接失败: %v", err)
		c.wsClient.Disconnect()
		return fmt.Errorf("mqtt connect: %w", err)
	}
	log.Printf("[INFO] [Start] MQTT 连接成功")

	c.updateStatus(func(s *ConnectorStatus) {
		s.WebSocketStatus = StatusConnected
		s.MQTTStatus = StatusConnected
		s.IsRunning = true
		s.StartTime = time.Now()
	})

	c.wg.Add(1)
	go c.messageLoop()

	log.Printf("[INFO] [Start] Connector 启动成功")

	return nil
}

// Stop 停止连接器，断开所有连接
func (c *connector) Stop() error {
	log.Printf("[INFO] [Stop] 正在停止 Connector")

	c.cancel()
	log.Printf("[DEBUG] [Stop] 上下文已取消")

	log.Printf("[INFO] [Stop] 正在断开 WebSocket")
	c.wsClient.Disconnect()
	log.Printf("[INFO] [Stop] WebSocket 已断开")

	log.Printf("[INFO] [Stop] 正在断开 MQTT")
	c.mqttClient.Disconnect(5 * time.Second)
	log.Printf("[INFO] [Stop] MQTT 已断开")

	log.Printf("[DEBUG] [Stop] 等待所有 goroutine 退出")
	c.wg.Wait()
	log.Printf("[DEBUG] [Stop] 所有 goroutine 已退出")

	c.updateStatus(func(s *ConnectorStatus) {
		s.WebSocketStatus = StatusDisconnected
		s.MQTTStatus = StatusDisconnected
		s.IsRunning = false
	})

	log.Printf("[INFO] [Stop] Connector 已停止")

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
	log.Printf("[INFO] [SubscribeExternalAgents] 订阅外部 Agents: %v", agentIDs)

	for _, agentID := range agentIDs {
		topic := protocol.BuildAgentTopic(agentID)
		if err := c.mqttClient.Subscribe(topic, c.config.Router.DefaultQoS, c.mqttMessageHandler); err != nil {
			return fmt.Errorf("subscribe to agent %s failed: %w", agentID, err)
		}

		c.subscribedMu.Lock()
		c.subscribedAgents[agentID] = true
		c.subscribedMu.Unlock()

		log.Printf("[INFO] [SubscribeExternalAgents] 已订阅外部 Agent: %s (topic: %s)", agentID, topic)
	}

	return nil
}

// SubscribeExternalGroups 订阅外部群组消息
func (c *connector) SubscribeExternalGroups(groupIDs []string) error {
	log.Printf("[INFO] [SubscribeExternalGroups] 订阅外部群组: %v", groupIDs)

	for _, groupID := range groupIDs {
		topic := protocol.BuildGroupTopic(groupID)
		if err := c.mqttClient.Subscribe(topic, c.config.Router.DefaultQoS, c.mqttMessageHandler); err != nil {
			return fmt.Errorf("subscribe to group %s failed: %w", groupID, err)
		}

		c.subscribedMu.Lock()
		c.subscribedGroups[groupID] = true
		c.subscribedMu.Unlock()

		log.Printf("[INFO] [SubscribeExternalGroups] 已订阅外部群组: %s (topic: %s)", groupID, topic)
	}

	return nil
}

// UnsubscribeExternalAgents 取消订阅外部 Agent
func (c *connector) UnsubscribeExternalAgents(agentIDs []string) error {
	log.Printf("[INFO] [UnsubscribeExternalAgents] 取消订阅外部 Agents: %v", agentIDs)

	for _, agentID := range agentIDs {
		topic := protocol.BuildAgentTopic(agentID)
		if err := c.mqttClient.Unsubscribe(topic); err != nil {
			return fmt.Errorf("unsubscribe from agent %s failed: %w", agentID, err)
		}

		c.subscribedMu.Lock()
		delete(c.subscribedAgents, agentID)
		c.subscribedMu.Unlock()

		log.Printf("[INFO] [UnsubscribeExternalAgents] 已取消订阅外部 Agent: %s", agentID)
	}

	return nil
}

// UnsubscribeExternalGroups 取消订阅外部群组
func (c *connector) UnsubscribeExternalGroups(groupIDs []string) error {
	log.Printf("[INFO] [UnsubscribeExternalGroups] 取消订阅外部群组: %v", groupIDs)

	for _, groupID := range groupIDs {
		topic := protocol.BuildGroupTopic(groupID)
		if err := c.mqttClient.Unsubscribe(topic); err != nil {
			return fmt.Errorf("unsubscribe from group %s failed: %w", groupID, err)
		}

		c.subscribedMu.Lock()
		delete(c.subscribedGroups, groupID)
		c.subscribedMu.Unlock()

		log.Printf("[INFO] [UnsubscribeExternalGroups] 已取消订阅外部群组: %s", groupID)
	}

	return nil
}

// OnMessage 注册消息处理函数
func (c *connector) OnMessage(handler MessageHandler) {
	c.msgHandlers = append(c.msgHandlers, handler)
	log.Printf("[DEBUG] [OnMessage] 消息处理器已注册, 当前处理器数量: %d", len(c.msgHandlers))
}

// OnError 注册错误处理函数
func (c *connector) OnError(handler ErrorHandler) {
	c.errHandlers = append(c.errHandlers, handler)
	log.Printf("[DEBUG] [OnError] 错误处理器已注册, 当前处理器数量: %d", len(c.errHandlers))
}

// OnStatusChange 注册状态变更处理函数
func (c *connector) OnStatusChange(handler StatusHandler) {
	c.statusHandlers = append(c.statusHandlers, handler)
	log.Printf("[DEBUG] [OnStatusChange] 状态变更处理器已注册, 当前处理器数量: %d", len(c.statusHandlers))
}

// mqttMessageHandler 统一的 MQTT 消息处理函数
func (c *connector) mqttMessageHandler(topic string, data []byte) {
	log.Printf("[DEBUG] [mqttMessageHandler] 收到 MQTT 消息, Topic=%s, Content=%s", topic, string(data))

	picoMsg, err := c.converter.MQTTToPico(topic, data)
	if err != nil {
		log.Printf("[ERROR] [mqttMessageHandler] MQTT 消息转换失败: %v", err)
		c.notifyError(err, ErrorContext{
			Source:    "mqtt",
			Operation: "convert",
		})
		return
	}

	if err := c.wsClient.SendPicoMessage(picoMsg); err != nil {
		log.Printf("[ERROR] [mqttMessageHandler] 转发到 WebSocket 失败: %v", err)
		c.notifyError(err, ErrorContext{
			Source:    "websocket",
			Operation: "send",
		})
		return
	}

	log.Printf("[DEBUG] [mqttMessageHandler] MQTT 消息已转发到 WebSocket, SessionID=%s", picoMsg.SessionID)
}

// messageLoop 消息处理循环
func (c *connector) messageLoop() {
	defer c.wg.Done()

	log.Printf("[INFO] [messageLoop] 消息循环已启动")

	ticker := time.NewTicker(c.config.WebSocket.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			log.Printf("[INFO] [messageLoop] 收到退出信号，消息循环结束")
			return

		case <-ticker.C:
			log.Printf("[DEBUG] [messageLoop] 发送 WebSocket ping")
			if err := c.wsClient.SendPing(); err != nil {
				log.Printf("[ERROR] [messageLoop] Ping 失败: %v", err)
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

			log.Printf("[DEBUG] [messageLoop] 收到 WebSocket 消息, Type=%s, SessionID=%s", picoMsg.Type, picoMsg.SessionID)

			// 过滤掉 pong 消息（心跳响应），不转发到 MQTT
			if picoMsg.Type == protocol.TypePong {
				log.Printf("[DEBUG] [messageLoop] 收到 pong 消息，不转发到 MQTT")
				continue
			}

			if err := c.forwardWebSocketToMQTT(picoMsg); err != nil {
				log.Printf("[ERROR] [messageLoop] 转发到 MQTT 失败: %v", err)
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

	log.Printf("[DEBUG] [forwardWebSocketToMQTT] 转发到 MQTT, Topic=%s, Content=%s", topic, string(payload))

	if err := c.mqttClient.Publish(topic, c.config.Router.DefaultQoS, false, payload); err != nil {
		return fmt.Errorf("publish to mqtt: %w", err)
	}

	log.Printf("[DEBUG] [forwardWebSocketToMQTT] 转发成功, Topic=%s", topic)
	return nil
}

// updateStatus 更新连接器状态
func (c *connector) updateStatus(fn func(*ConnectorStatus)) {
	c.statusMu.Lock()
	oldStatus := c.status
	fn(&c.status)
	newStatus := c.status
	c.statusMu.Unlock()

	log.Printf("[DEBUG] [updateStatus] 状态变更: WS=%s->%s, MQTT=%s->%s, Running=%v->%v",
		oldStatus.WebSocketStatus, newStatus.WebSocketStatus,
		oldStatus.MQTTStatus, newStatus.MQTTStatus,
		oldStatus.IsRunning, newStatus.IsRunning)

	for _, handler := range c.statusHandlers {
		handler(oldStatus.WebSocketStatus, newStatus.WebSocketStatus)
	}
}

// notifyError 通知所有错误处理函数
func (c *connector) notifyError(err error, ctx ErrorContext) {
	ctx.Timestamp = time.Now().Unix()
	log.Printf("[ERROR] [notifyError] 发生错误: Source=%s, Operation=%s, Error=%v", ctx.Source, ctx.Operation, err)

	for _, handler := range c.errHandlers {
		handler(err, ctx)
	}
}
