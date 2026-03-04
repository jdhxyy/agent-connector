package mqtt

import (
	"context"
	"fmt"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/jdhxyy/agent-connector/protocol"
)

type Client struct {
	config          *Config
	client          mqtt.Client
	mu              sync.RWMutex
	status          Status
	handlers        map[string]protocol.MessageHandler
	subscriptions   map[string]byte
	connectionLost  func(clientID string, err error)
	reconnecting    func(clientID string)
	onConnect       func(clientID string)
}

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

type Status int

const (
	StatusDisconnected Status = iota
	StatusConnecting
	StatusConnected
)

func NewClient(config *Config) *Client {
	return &Client{
		config:        config,
		status:        StatusDisconnected,
		handlers:      make(map[string]protocol.MessageHandler),
		subscriptions: make(map[string]byte),
	}
}

func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status == StatusConnected {
		return fmt.Errorf("already connected")
	}

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

	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		c.mu.Lock()
		c.status = StatusDisconnected
		c.mu.Unlock()
		if c.connectionLost != nil {
			c.connectionLost(c.config.ClientID, err)
		}
	})

	opts.SetReconnectingHandler(func(client mqtt.Client, opts *mqtt.ClientOptions) {
		if c.reconnecting != nil {
			c.reconnecting(c.config.ClientID)
		}
	})

	opts.SetOnConnectHandler(func(client mqtt.Client) {
		c.mu.Lock()
		c.status = StatusConnected
		c.mu.Unlock()
		if c.onConnect != nil {
			c.onConnect(c.config.ClientID)
		}
		c.resubscribe()
	})

	c.client = mqtt.NewClient(opts)

	token := c.client.Connect()
	select {
	case <-token.Done():
		if token.Error() != nil {
			c.status = StatusDisconnected
			return fmt.Errorf("mqtt connection failed: %w", token.Error())
		}
	case <-ctx.Done():
		c.status = StatusDisconnected
		return fmt.Errorf("mqtt connection timeout: %w", ctx.Err())
	}

	c.status = StatusConnected
	return nil
}

func (c *Client) Disconnect(waitTime time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status == StatusDisconnected {
		return nil
	}

	c.client.Disconnect(uint(waitTime.Milliseconds()))
	c.status = StatusDisconnected
	return nil
}

func (c *Client) Subscribe(topic string, qos byte, handler protocol.MessageHandler) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status != StatusConnected {
		return fmt.Errorf("not connected")
	}

	token := c.client.Subscribe(topic, qos, func(client mqtt.Client, msg mqtt.Message) {
		genericMsg := &protocol.GenericMessage{
			ID:        fmt.Sprintf("%d", msg.MessageID()),
			MsgType:   "mqtt",
			Payload:   msg.Payload(),
			Timestamp: time.Now(),
			Metadata: map[string]string{
				"topic": msg.Topic(),
				"qos":   fmt.Sprintf("%d", msg.Qos()),
			},
		}
		if handler != nil {
			handler(genericMsg)
		}
	})

	token.Wait()
	if token.Error() != nil {
		return fmt.Errorf("subscribe failed: %w", token.Error())
	}

	c.handlers[topic] = handler
	c.subscriptions[topic] = qos

	return nil
}

func (c *Client) SubscribeMultiple(topics map[string]byte, handler protocol.MessageHandler) error {
	for topic, qos := range topics {
		if err := c.Subscribe(topic, qos, handler); err != nil {
			return fmt.Errorf("subscribe to %s failed: %w", topic, err)
		}
	}
	return nil
}

func (c *Client) Unsubscribe(topics ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status != StatusConnected {
		return fmt.Errorf("not connected")
	}

	token := c.client.Unsubscribe(topics...)
	token.Wait()
	if token.Error() != nil {
		return fmt.Errorf("unsubscribe failed: %w", token.Error())
	}

	for _, topic := range topics {
		delete(c.handlers, topic)
		delete(c.subscriptions, topic)
	}

	return nil
}

func (c *Client) Publish(topic string, qos byte, retained bool, payload interface{}) error {
	c.mu.RLock()
	if c.status != StatusConnected {
		c.mu.RUnlock()
		return fmt.Errorf("not connected")
	}
	client := c.client
	c.mu.RUnlock()

	token := client.Publish(topic, qos, retained, payload)
	token.Wait()
	if token.Error() != nil {
		return fmt.Errorf("publish failed: %w", token.Error())
	}

	return nil
}

func (c *Client) resubscribe() {
	c.mu.RLock()
	subscriptions := make(map[string]byte)
	for topic, qos := range c.subscriptions {
		subscriptions[topic] = qos
	}
	c.mu.RUnlock()

	for topic, qos := range subscriptions {
		token := c.client.Subscribe(topic, qos, nil)
		token.Wait()
	}
}

func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status == StatusConnected
}

func (c *Client) GetStatus() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

func (c *Client) SetConnectionLostHandler(handler func(clientID string, err error)) {
	c.connectionLost = handler
}

func (c *Client) SetReconnectingHandler(handler func(clientID string)) {
	c.reconnecting = handler
}

func (c *Client) SetOnConnectHandler(handler func(clientID string)) {
	c.onConnect = handler
}
