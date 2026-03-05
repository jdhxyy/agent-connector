package mqtt

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// MessageHandler MQTT 消息处理函数类型
type MessageHandler func(topic string, data []byte)

// Client MQTT 客户端
type Client struct {
	config         *Config
	client         mqtt.Client
	mu             sync.RWMutex
	status         Status
	handlers       map[string]MessageHandler
	subscriptions  map[string]byte
	connectionLost func(clientID string, err error)
	reconnecting   func(clientID string)
	onConnect      func(clientID string)
}

// Config MQTT 客户端配置
type Config struct {
	BrokerURL            string
	ClientID             string
	Username             string
	Password             string
	KeepAlive            time.Duration
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
	log.Printf("[INFO] [NewClient] 正在创建 MQTT 客户端, ClientID=%s, Broker=%s",
		config.ClientID, config.BrokerURL)

	client := &Client{
		config:        config,
		status:        StatusDisconnected,
		handlers:      make(map[string]MessageHandler),
		subscriptions: make(map[string]byte),
	}

	log.Printf("[INFO] [NewClient] MQTT 客户端创建成功, ClientID=%s", config.ClientID)

	return client
}

// Connect 连接到 MQTT Broker
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status == StatusConnected {
		log.Printf("[WARN] [Connect] MQTT 客户端已连接, ClientID=%s", c.config.ClientID)
		return fmt.Errorf("already connected")
	}

	log.Printf("[INFO] [Connect] 正在连接 MQTT Broker, ClientID=%s, Broker=%s",
		c.config.ClientID, c.config.BrokerURL)

	c.status = StatusConnecting

	opts := mqtt.NewClientOptions()
	opts.AddBroker(c.config.BrokerURL)
	opts.SetClientID(c.config.ClientID)
	opts.SetUsername(c.config.Username)
	opts.SetPassword(c.config.Password)
	opts.SetKeepAlive(c.config.KeepAlive)
	opts.SetCleanSession(c.config.CleanSession)
	opts.SetAutoReconnect(c.config.AutoReconnect)
	opts.SetMaxReconnectInterval(c.config.MaxReconnectInterval)

	log.Printf("[DEBUG] [Connect] MQTT 配置: KeepAlive=%v, CleanSession=%v, AutoReconnect=%v",
		c.config.KeepAlive, c.config.CleanSession, c.config.AutoReconnect)

	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		log.Printf("[ERROR] [Connect] MQTT 连接丢失, ClientID=%s, Error=%v",
			c.config.ClientID, err)
		c.mu.Lock()
		c.status = StatusDisconnected
		c.mu.Unlock()
		if c.connectionLost != nil {
			c.connectionLost(c.config.ClientID, err)
		}
	})

	opts.SetReconnectingHandler(func(client mqtt.Client, opts *mqtt.ClientOptions) {
		log.Printf("[INFO] [Connect] MQTT 正在重连, ClientID=%s", c.config.ClientID)
		if c.reconnecting != nil {
			c.reconnecting(c.config.ClientID)
		}
	})

	opts.SetOnConnectHandler(func(client mqtt.Client) {
		log.Printf("[INFO] [Connect] MQTT 连接成功, ClientID=%s", c.config.ClientID)
		c.mu.Lock()
		c.status = StatusConnected
		c.mu.Unlock()
		if c.onConnect != nil {
			c.onConnect(c.config.ClientID)
		}
		c.resubscribe()
	})

	c.client = mqtt.NewClient(opts)

	log.Printf("[DEBUG] [Connect] 正在执行 MQTT 连接...")
	token := c.client.Connect()

	select {
	case <-token.Done():
		if token.Error() != nil {
			c.status = StatusDisconnected
			log.Printf("[ERROR] [Connect] MQTT 连接失败: %v", token.Error())
			return fmt.Errorf("mqtt connection failed: %w", token.Error())
		}
	case <-ctx.Done():
		c.status = StatusDisconnected
		log.Printf("[ERROR] [Connect] MQTT 连接超时: %v", ctx.Err())
		return fmt.Errorf("mqtt connection timeout: %w", ctx.Err())
	}

	c.status = StatusConnected
	log.Printf("[INFO] [Connect] MQTT 连接成功, ClientID=%s", c.config.ClientID)

	return nil
}

// Disconnect 断开 MQTT 连接
func (c *Client) Disconnect(waitTime time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status == StatusDisconnected {
		log.Printf("[DEBUG] [Disconnect] MQTT 客户端已断开, ClientID=%s", c.config.ClientID)
		return nil
	}

	log.Printf("[INFO] [Disconnect] 正在断开 MQTT 连接, ClientID=%s, WaitTime=%v",
		c.config.ClientID, waitTime)

	c.client.Disconnect(uint(waitTime.Milliseconds()))
	c.status = StatusDisconnected

	log.Printf("[INFO] [Disconnect] MQTT 连接已断开, ClientID=%s", c.config.ClientID)

	return nil
}

// Subscribe 订阅 MQTT 主题
func (c *Client) Subscribe(topic string, qos byte, handler MessageHandler) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status != StatusConnected {
		log.Printf("[ERROR] [Subscribe] MQTT 未连接, 无法订阅主题: %s", topic)
		return fmt.Errorf("not connected")
	}

	log.Printf("[INFO] [Subscribe] 正在订阅主题: %s (QoS=%d), ClientID=%s",
		topic, qos, c.config.ClientID)

	token := c.client.Subscribe(topic, qos, func(client mqtt.Client, msg mqtt.Message) {
		log.Printf("[DEBUG] [Subscribe] 收到 MQTT 消息, Topic=%s, QoS=%d, Payload=%s",
			msg.Topic(), msg.Qos(), string(msg.Payload()))

		if handler != nil {
			handler(msg.Topic(), msg.Payload())
		}
	})

	token.Wait()
	if token.Error() != nil {
		log.Printf("[ERROR] [Subscribe] 订阅失败, Topic=%s: %v", topic, token.Error())
		return fmt.Errorf("subscribe failed: %w", token.Error())
	}

	c.handlers[topic] = handler
	c.subscriptions[topic] = qos

	log.Printf("[INFO] [Subscribe] 订阅成功, Topic=%s (QoS=%d), ClientID=%s",
		topic, qos, c.config.ClientID)

	return nil
}

// SubscribeMultiple 批量订阅多个主题
func (c *Client) SubscribeMultiple(topics map[string]byte, handler MessageHandler) error {
	log.Printf("[INFO] [SubscribeMultiple] 批量订阅 %d 个主题, ClientID=%s",
		len(topics), c.config.ClientID)

	for topic, qos := range topics {
		if err := c.Subscribe(topic, qos, handler); err != nil {
			log.Printf("[ERROR] [SubscribeMultiple] 订阅主题 %s 失败: %v", topic, err)
			return fmt.Errorf("subscribe to %s failed: %w", topic, err)
		}
	}

	log.Printf("[INFO] [SubscribeMultiple] 批量订阅完成, ClientID=%s", c.config.ClientID)

	return nil
}

// Unsubscribe 取消订阅 MQTT 主题
func (c *Client) Unsubscribe(topics ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status != StatusConnected {
		log.Printf("[ERROR] [Unsubscribe] MQTT 未连接, 无法取消订阅")
		return fmt.Errorf("not connected")
	}

	log.Printf("[INFO] [Unsubscribe] 正在取消订阅 %d 个主题: %v, ClientID=%s",
		len(topics), topics, c.config.ClientID)

	token := c.client.Unsubscribe(topics...)
	token.Wait()
	if token.Error() != nil {
		log.Printf("[ERROR] [Unsubscribe] 取消订阅失败: %v", token.Error())
		return fmt.Errorf("unsubscribe failed: %w", token.Error())
	}

	for _, topic := range topics {
		delete(c.handlers, topic)
		delete(c.subscriptions, topic)
	}

	log.Printf("[INFO] [Unsubscribe] 取消订阅成功, ClientID=%s", c.config.ClientID)

	return nil
}

// Publish 发布 MQTT 消息
func (c *Client) Publish(topic string, qos byte, retained bool, payload interface{}) error {
	c.mu.RLock()
	if c.status != StatusConnected {
		c.mu.RUnlock()
		log.Printf("[ERROR] [Publish] MQTT 未连接, 无法发布消息到主题: %s", topic)
		return fmt.Errorf("not connected")
	}
	client := c.client
	c.mu.RUnlock()

	log.Printf("[DEBUG] [Publish] 正在发布消息, Topic=%s, QoS=%d, Retained=%v, Payload=%v",
		topic, qos, retained, payload)

	token := client.Publish(topic, qos, retained, payload)
	token.Wait()
	if token.Error() != nil {
		log.Printf("[ERROR] [Publish] 发布失败, Topic=%s: %v", topic, token.Error())
		return fmt.Errorf("publish failed: %w", token.Error())
	}

	log.Printf("[DEBUG] [Publish] 发布成功, Topic=%s", topic)

	return nil
}

// resubscribe 重新订阅所有已订阅的主题
func (c *Client) resubscribe() {
	c.mu.RLock()
	subscriptions := make(map[string]byte)
	for topic, qos := range c.subscriptions {
		subscriptions[topic] = qos
	}
	handlers := make(map[string]MessageHandler)
	for topic, handler := range c.handlers {
		handlers[topic] = handler
	}
	c.mu.RUnlock()

	if len(subscriptions) == 0 {
		return
	}

	log.Printf("[INFO] [resubscribe] 正在重新订阅 %d 个主题, ClientID=%s",
		len(subscriptions), c.config.ClientID)

	for topic, qos := range subscriptions {
		log.Printf("[DEBUG] [resubscribe] 重新订阅主题: %s (QoS=%d)", topic, qos)
		handler := handlers[topic]
		token := c.client.Subscribe(topic, qos, func(client mqtt.Client, msg mqtt.Message) {
			if handler != nil {
				handler(msg.Topic(), msg.Payload())
			}
		})
		token.Wait()
		if token.Error() != nil {
			log.Printf("[ERROR] [resubscribe] 重新订阅失败, Topic=%s: %v", topic, token.Error())
		}
	}

	log.Printf("[INFO] [resubscribe] 重新订阅完成, ClientID=%s", c.config.ClientID)
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
	log.Printf("[DEBUG] [SetConnectionLostHandler] 设置连接丢失回调, ClientID=%s", c.config.ClientID)
	c.connectionLost = handler
}

// SetReconnectingHandler 设置重连回调
func (c *Client) SetReconnectingHandler(handler func(clientID string)) {
	log.Printf("[DEBUG] [SetReconnectingHandler] 设置重连回调, ClientID=%s", c.config.ClientID)
	c.reconnecting = handler
}

// SetOnConnectHandler 设置连接成功回调
func (c *Client) SetOnConnectHandler(handler func(clientID string)) {
	log.Printf("[DEBUG] [SetOnConnectHandler] 设置连接成功回调, ClientID=%s", c.config.ClientID)
	c.onConnect = handler
}
