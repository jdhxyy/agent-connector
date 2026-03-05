package mqtt

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
)

// MessageHandler MQTT 消息处理函数类型
type MessageHandler func(topic string, data []byte)

// Subscription 订阅配置
type Subscription struct {
	Topic   string
	QoS     byte
	NoLocal bool // 设置为 true 时，不会接收到自己发送的消息
}

// Client MQTT 客户端
type Client struct {
	config         *Config
	manager        *autopaho.ConnectionManager
	mu             sync.RWMutex
	status         Status
	handlers       map[string]MessageHandler
	subscriptions  map[string]*Subscription
	connectionLost func(clientID string, err error)
	reconnecting   func(clientID string)
	onConnect      func(clientID string)
	ctx            context.Context
	cancel         context.CancelFunc
}

// Config MQTT 客户端配置
type Config struct {
	BrokerURL            string
	ClientID             string
	Username             string
	Password             string
	KeepAlive            uint16
	ConnectTimeout       time.Duration
	CleanSession         bool
	AutoReconnect        bool
	MaxReconnectInterval time.Duration
}

// Status 连接状态枚举
type Status int

const (
	StatusDisconnected Status = iota
	StatusConnecting
	StatusConnected
)

// NewClient 创建新的 MQTT 客户端
func NewClient(config *Config) *Client {
	log.Printf("[INFO] [NewClient] Creating MQTT client, ClientID=%s, Broker=%s",
		config.ClientID, config.BrokerURL)

	ctx, cancel := context.WithCancel(context.Background())

	client := &Client{
		config:        config,
		status:        StatusDisconnected,
		handlers:      make(map[string]MessageHandler),
		subscriptions: make(map[string]*Subscription),
		ctx:           ctx,
		cancel:        cancel,
	}

	log.Printf("[INFO] [NewClient] MQTT client created successfully, ClientID=%s", config.ClientID)

	return client
}

// Connect 连接到 MQTT Broker
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status == StatusConnected {
		log.Printf("[WARN] [Connect] MQTT client already connected, ClientID=%s", c.config.ClientID)
		return fmt.Errorf("already connected")
	}

	log.Printf("[INFO] [Connect] Connecting to MQTT Broker, ClientID=%s, Broker=%s",
		c.config.ClientID, c.config.BrokerURL)

	c.status = StatusConnecting

	// 解析 Broker URL
	serverURL, err := url.Parse(c.config.BrokerURL)
	if err != nil {
		c.status = StatusDisconnected
		log.Printf("[ERROR] [Connect] Failed to parse Broker URL: %v", err)
		return fmt.Errorf("parse broker url: %w", err)
	}

	// 创建 autopaho 配置
	cfg := autopaho.ClientConfig{
		ServerUrls:                    []*url.URL{serverURL},
		ConnectUsername:               c.config.Username,
		ConnectPassword:               []byte(c.config.Password),
		CleanStartOnInitialConnection: c.config.CleanSession,
		KeepAlive:                     c.config.KeepAlive,
		ConnectTimeout:                c.config.ConnectTimeout,
	}

	// 设置 ClientID 通过 ConnectPacketBuilder
	clientID := c.config.ClientID
	cfg.ConnectPacketBuilder = func(connect *paho.Connect, url *url.URL) (*paho.Connect, error) {
		connect.ClientID = clientID
		return connect, nil
	}

	// 设置连接成功回调
	cfg.OnConnectionUp = func(cm *autopaho.ConnectionManager, connAck *paho.Connack) {
		log.Printf("[INFO] [Connect] MQTT connected successfully, ClientID=%s", c.config.ClientID)
		c.mu.Lock()
		c.status = StatusConnected
		c.mu.Unlock()
		if c.onConnect != nil {
			c.onConnect(c.config.ClientID)
		}
	}

	// 设置连接关闭回调
	cfg.OnConnectError = func(err error) {
		log.Printf("[ERROR] [Connect] MQTT connection error: %v", err)
		c.mu.Lock()
		c.status = StatusDisconnected
		c.mu.Unlock()
		if c.connectionLost != nil {
			c.connectionLost(c.config.ClientID, err)
		}
	}

	// 设置消息接收回调
	cfg.OnPublishReceived = []func(paho.PublishReceived) (bool, error){
		func(pr paho.PublishReceived) (bool, error) {
			msg := pr.Packet
			topic := msg.Topic
			payload := msg.Payload

			log.Printf("[DEBUG] [Connect] Received MQTT message, Topic=%s, QoS=%d, Payload=%s",
				topic, msg.QoS, string(payload))

			c.mu.RLock()
			handler := c.handlers[topic]
			c.mu.RUnlock()

			if handler != nil {
				handler(topic, payload)
			}

			return true, nil
		},
	}

	// 创建连接管理器
	manager, err := autopaho.NewConnection(c.ctx, cfg)
	if err != nil {
		c.status = StatusDisconnected
		log.Printf("[ERROR] [Connect] Failed to create connection manager: %v", err)
		return fmt.Errorf("create connection manager: %w", err)
	}

	c.manager = manager

	// 等待连接建立
	connectCtx, cancel := context.WithTimeout(ctx, c.config.ConnectTimeout)
	defer cancel()

	if err := manager.AwaitConnection(connectCtx); err != nil {
		c.status = StatusDisconnected
		log.Printf("[ERROR] [Connect] Failed to await connection: %v", err)
		return fmt.Errorf("await connection: %w", err)
	}

	log.Printf("[INFO] [Connect] MQTT connection established, ClientID=%s", c.config.ClientID)

	return nil
}

// Disconnect 断开 MQTT 连接
func (c *Client) Disconnect(waitTime time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status == StatusDisconnected {
		log.Printf("[DEBUG] [Disconnect] MQTT client already disconnected, ClientID=%s", c.config.ClientID)
		return nil
	}

	log.Printf("[INFO] [Disconnect] Disconnecting MQTT, ClientID=%s, WaitTime=%v",
		c.config.ClientID, waitTime)

	// 取消上下文，停止连接管理器
	c.cancel()

	// 断开连接
	ctx, cancel := context.WithTimeout(context.Background(), waitTime)
	defer cancel()

	if err := c.manager.Disconnect(ctx); err != nil {
		log.Printf("[WARN] [Disconnect] Failed to disconnect: %v", err)
	}

	c.status = StatusDisconnected

	log.Printf("[INFO] [Disconnect] MQTT disconnected, ClientID=%s", c.config.ClientID)

	return nil
}

// Subscribe 订阅 MQTT 主题（默认 NoLocal=true，不接收自己发送的消息）
func (c *Client) Subscribe(topic string, qos byte, handler MessageHandler) error {
	return c.SubscribeWithOptions(topic, qos, true, handler)
}

// SubscribeWithOptions 使用指定选项订阅 MQTT 主题
// noLocal: 设置为 true 时，不会接收到自己发送的消息
func (c *Client) SubscribeWithOptions(topic string, qos byte, noLocal bool, handler MessageHandler) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status != StatusConnected {
		log.Printf("[ERROR] [Subscribe] MQTT not connected, cannot subscribe to topic: %s", topic)
		return fmt.Errorf("not connected")
	}

	log.Printf("[INFO] [Subscribe] Subscribing to topic: %s (QoS=%d, NoLocal=%v), ClientID=%s",
		topic, qos, noLocal, c.config.ClientID)

	// 使用 paho.golang 的 SubscribeOptions 设置 NoLocal
	subscribePacket := &paho.Subscribe{
		Subscriptions: []paho.SubscribeOptions{
			{
				Topic:             topic,
				QoS:               qos,
				NoLocal:           noLocal,
				RetainAsPublished: false,
				RetainHandling:    0,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	suback, err := c.manager.Subscribe(ctx, subscribePacket)
	if err != nil {
		log.Printf("[ERROR] [Subscribe] Subscribe failed, Topic=%s: %v", topic, err)
		return fmt.Errorf("subscribe failed: %w", err)
	}

	// 检查订阅结果
	if len(suback.Reasons) > 0 && suback.Reasons[0] >= 0x80 {
		log.Printf("[ERROR] [Subscribe] Subscribe rejected, Topic=%s, ReasonCode=%d", topic, suback.Reasons[0])
		return fmt.Errorf("subscribe rejected, reason code: %d", suback.Reasons[0])
	}

	c.handlers[topic] = handler
	c.subscriptions[topic] = &Subscription{Topic: topic, QoS: qos, NoLocal: noLocal}

	log.Printf("[INFO] [Subscribe] Subscribe successful, Topic=%s (QoS=%d, NoLocal=%v), ClientID=%s",
		topic, qos, noLocal, c.config.ClientID)

	return nil
}

// SubscribeMultiple 批量订阅多个主题
func (c *Client) SubscribeMultiple(topics map[string]byte, handler MessageHandler) error {
	log.Printf("[INFO] [SubscribeMultiple] Batch subscribing %d topics, ClientID=%s",
		len(topics), c.config.ClientID)

	for topic, qos := range topics {
		if err := c.Subscribe(topic, qos, handler); err != nil {
			log.Printf("[ERROR] [SubscribeMultiple] Failed to subscribe to topic %s: %v", topic, err)
			return fmt.Errorf("subscribe to %s failed: %w", topic, err)
		}
	}

	log.Printf("[INFO] [SubscribeMultiple] Batch subscribe completed, ClientID=%s", c.config.ClientID)

	return nil
}

// Unsubscribe 取消订阅 MQTT 主题
func (c *Client) Unsubscribe(topics ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status != StatusConnected {
		log.Printf("[ERROR] [Unsubscribe] MQTT not connected, cannot unsubscribe")
		return fmt.Errorf("not connected")
	}

	log.Printf("[INFO] [Unsubscribe] Unsubscribing %d topics: %v, ClientID=%s",
		len(topics), topics, c.config.ClientID)

	unsubscribePacket := &paho.Unsubscribe{
		Topics: topics,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := c.manager.Unsubscribe(ctx, unsubscribePacket)
	if err != nil {
		log.Printf("[ERROR] [Unsubscribe] Unsubscribe failed: %v", err)
		return fmt.Errorf("unsubscribe failed: %w", err)
	}

	for _, topic := range topics {
		delete(c.handlers, topic)
		delete(c.subscriptions, topic)
	}

	log.Printf("[INFO] [Unsubscribe] Unsubscribe successful, ClientID=%s", c.config.ClientID)

	return nil
}

// Publish 发布 MQTT 消息
func (c *Client) Publish(topic string, qos byte, retained bool, payload interface{}) error {
	c.mu.RLock()
	if c.status != StatusConnected {
		c.mu.RUnlock()
		log.Printf("[ERROR] [Publish] MQTT not connected, cannot publish to topic: %s", topic)
		return fmt.Errorf("not connected")
	}
	manager := c.manager
	c.mu.RUnlock()

	log.Printf("[DEBUG] [Publish] Publishing message, Topic=%s, QoS=%d, Retained=%v",
		topic, qos, retained)

	var payloadBytes []byte
	switch v := payload.(type) {
	case []byte:
		payloadBytes = v
	case string:
		payloadBytes = []byte(v)
	default:
		payloadBytes = []byte(fmt.Sprintf("%v", v))
	}

	publishPacket := &paho.Publish{
		Topic:   topic,
		QoS:     qos,
		Retain:  retained,
		Payload: payloadBytes,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := manager.Publish(ctx, publishPacket)
	if err != nil {
		log.Printf("[ERROR] [Publish] Publish failed, Topic=%s: %v", topic, err)
		return fmt.Errorf("publish failed: %w", err)
	}

	log.Printf("[DEBUG] [Publish] Publish successful, Topic=%s", topic)

	return nil
}

// IsConnected 检查是否已连接
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status == StatusConnected
}

// GetStatus 获取连接状态
func (c *Client) GetStatus() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// SetConnectionLostHandler 设置连接丢失回调
func (c *Client) SetConnectionLostHandler(handler func(clientID string, err error)) {
	log.Printf("[DEBUG] [SetConnectionLostHandler] Setting connection lost handler, ClientID=%s", c.config.ClientID)
	c.connectionLost = handler
}

// SetReconnectingHandler 设置重连回调
func (c *Client) SetReconnectingHandler(handler func(clientID string)) {
	log.Printf("[DEBUG] [SetReconnectingHandler] Setting reconnecting handler, ClientID=%s", c.config.ClientID)
	c.reconnecting = handler
}

// SetOnConnectHandler 设置连接成功回调
func (c *Client) SetOnConnectHandler(handler func(clientID string)) {
	log.Printf("[DEBUG] [SetOnConnectHandler] Setting on connect handler, ClientID=%s", c.config.ClientID)
	c.onConnect = handler
}
