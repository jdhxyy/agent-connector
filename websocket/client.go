package websocket

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jdhxyy/agent-connector/protocol"
)

// Client WebSocket 客户端
type Client struct {
	config    *Config
	conn      *websocket.Conn
	mu        sync.RWMutex
	status    Status
	msgChan   chan protocol.PicoMessage
	errChan   chan error
	stopChan  chan struct{}
	wg        sync.WaitGroup
	handlers  []protocol.MessageHandler
	sessionID string
}

// Config WebSocket 客户端配置
type Config struct {
	BaseURL           string
	Token             string
	SessionID         string
	HandshakeTimeout  time.Duration
	PingInterval      time.Duration
	ReconnectMaxRetry int
	ReconnectDelay    time.Duration
}

// Status 连接状态枚举
type Status int

const (
	StatusDisconnected Status = iota
	StatusConnecting
	StatusConnected
	StatusReconnecting
)

// NewClient 创建新的 WebSocket 客户端
func NewClient(config *Config) *Client {
	sessionID := config.SessionID
	if sessionID == "" {
		sessionID = generateSessionID()
	}

	return &Client{
		config:    config,
		status:    StatusDisconnected,
		msgChan:   make(chan protocol.PicoMessage, 100),
		errChan:   make(chan error, 10),
		stopChan:  make(chan struct{}),
		sessionID: sessionID,
	}
}

// Connect 连接到 WebSocket 服务器
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status == StatusConnected {
		return fmt.Errorf("already connected")
	}

	c.status = StatusConnecting

	wsURL := fmt.Sprintf("%s/pico/ws?session_id=%s", c.config.BaseURL, c.sessionID)
	u, err := url.Parse(wsURL)
	if err != nil {
		c.status = StatusDisconnected
		return fmt.Errorf("invalid URL: %w", err)
	}

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+c.config.Token)

	dialer := websocket.Dialer{
		HandshakeTimeout: c.config.HandshakeTimeout,
	}

	conn, resp, err := dialer.DialContext(ctx, u.String(), headers)
	if err != nil {
		c.status = StatusDisconnected
		if resp != nil {
			return fmt.Errorf("connection failed (status %d): %w", resp.StatusCode, err)
		}
		return fmt.Errorf("connection failed: %w", err)
	}

	c.conn = conn
	c.status = StatusConnected

	c.wg.Add(1)
	go c.readLoop()

	return nil
}

// Disconnect 断开 WebSocket 连接
func (c *Client) Disconnect() error {
	c.mu.Lock()
	if c.status == StatusDisconnected {
		c.mu.Unlock()
		return nil
	}
	c.status = StatusDisconnected
	c.mu.Unlock()

	close(c.stopChan)

	if c.conn != nil {
		c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		c.conn.Close()
	}

	c.wg.Wait()
	return nil
}

// SendMessage 发送文本消息（便捷方法）
func (c *Client) SendMessage(content string) error {
	msg := protocol.NewTextMessage(content)
	return c.SendPicoMessage(msg)
}

// SendPicoMessage 发送 PicoMessage
func (c *Client) SendPicoMessage(msg protocol.PicoMessage) error {
	c.mu.RLock()
	if c.status != StatusConnected {
		c.mu.RUnlock()
		return fmt.Errorf("not connected")
	}
	conn := c.conn
	c.mu.RUnlock()

	data, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("write message: %w", err)
	}

	return nil
}

// SendPing 发送心跳 ping
func (c *Client) SendPing() error {
	msg := protocol.NewPing()
	return c.SendPicoMessage(msg)
}

// ReceiveMessage 接收消息（阻塞方法）
func (c *Client) ReceiveMessage(timeout time.Duration) (protocol.PicoMessage, error) {
	select {
	case msg := <-c.msgChan:
		return msg, nil
	case err := <-c.errChan:
		return protocol.PicoMessage{}, err
	case <-time.After(timeout):
		return protocol.PicoMessage{}, fmt.Errorf("receive timeout")
	}
}

// readLoop 消息读取循环
func (c *Client) readLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.stopChan:
			return
		default:
		}

		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				select {
				case c.errChan <- fmt.Errorf("connection closed unexpectedly: %w", err):
				default:
				}
			}
			return
		}

		var msg protocol.PicoMessage
		if err := msg.FromJSON(data); err != nil {
			select {
			case c.errChan <- fmt.Errorf("unmarshal message: %w", err):
			default:
			}
			continue
		}

		if msg.SessionID != "" {
			c.sessionID = msg.SessionID
		}

		select {
		case c.msgChan <- msg:
		case <-c.stopChan:
			return
		}
	}
}

// GetSessionID 获取会话 ID
func (c *Client) GetSessionID() string {
	return c.sessionID
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

// generateSessionID 生成唯一的会话 ID
func generateSessionID() string {
	return fmt.Sprintf("session-%d", time.Now().UnixNano())
}
