package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jdhxyy/agent-connector/protocol"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func createMockServer(t *testing.T) (*httptest.Server, chan protocol.PicoMessage, chan protocol.PicoMessage) {
	receivedMessages := make(chan protocol.PicoMessage, 10)
	sendMessages := make(chan protocol.PicoMessage, 10)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证 token
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			t.Log("Missing or invalid Authorization header")
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("Upgrade error: %v", err)
			return
		}
		defer conn.Close()

		// 读取消息
		go func() {
			for {
				_, data, err := conn.ReadMessage()
				if err != nil {
					return
				}
				var msg protocol.PicoMessage
				if err := msg.FromJSON(data); err != nil {
					t.Logf("Unmarshal error: %v", err)
					continue
				}
				receivedMessages <- msg

				// 如果是 ping，回复 pong
				if msg.Type == protocol.TypePing {
					pong := protocol.NewMessage(protocol.TypePong, nil)
					pongData, _ := pong.ToJSON()
					conn.WriteMessage(websocket.TextMessage, pongData)
				}
			}
		}()

		// 发送消息
		for msg := range sendMessages {
			data, _ := msg.ToJSON()
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				t.Logf("Write error: %v", err)
				return
			}
		}
	}))

	return server, receivedMessages, sendMessages
}

func TestNewClient(t *testing.T) {
	config := &Config{
		BaseURL:           "ws://localhost:8080",
		Token:             "test-token",
		SessionID:         "test-session",
		HandshakeTimeout:  10 * time.Second,
		PingInterval:      30 * time.Second,
		ReconnectMaxRetry: 10,
		ReconnectDelay:    3 * time.Second,
	}

	client := NewClient(config)
	if client == nil {
		t.Fatal("NewClient() returned nil")
	}

	if client.GetSessionID() != "test-session" {
		t.Errorf("SessionID = %v, want %v", client.GetSessionID(), "test-session")
	}

	if client.GetStatus() != StatusDisconnected {
		t.Errorf("Initial status = %v, want %v", client.GetStatus(), StatusDisconnected)
	}
}

func TestNewClient_AutoGenerateSessionID(t *testing.T) {
	config := &Config{
		BaseURL: "ws://localhost:8080",
		Token:   "test-token",
		// SessionID 为空，应该自动生成
	}

	client := NewClient(config)
	if client == nil {
		t.Fatal("NewClient() returned nil")
	}

	if client.GetSessionID() == "" {
		t.Error("SessionID should be auto-generated")
	}

	if !strings.HasPrefix(client.GetSessionID(), "session-") {
		t.Errorf("SessionID should start with 'session-', got %v", client.GetSessionID())
	}
}

func TestClient_Connect(t *testing.T) {
	server, _, _ := createMockServer(t)
	defer server.Close()

	// 将 http:// 替换为 ws://
	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)

	config := &Config{
		BaseURL:          wsURL,
		Token:            "test-token",
		HandshakeTimeout: 5 * time.Second,
	}

	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	if !client.IsConnected() {
		t.Error("Client should be connected")
	}

	if client.GetStatus() != StatusConnected {
		t.Errorf("Status = %v, want %v", client.GetStatus(), StatusConnected)
	}

	// 清理
	client.Disconnect()
}

func TestClient_Connect_InvalidURL(t *testing.T) {
	config := &Config{
		BaseURL:          "://invalid-url",
		Token:            "test-token",
		HandshakeTimeout: 5 * time.Second,
	}

	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	if err == nil {
		t.Error("Connect() should return error for invalid URL")
	}
}

func TestClient_Connect_AlreadyConnected(t *testing.T) {
	server, _, _ := createMockServer(t)
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)

	config := &Config{
		BaseURL:          wsURL,
		Token:            "test-token",
		HandshakeTimeout: 5 * time.Second,
	}

	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 第一次连接
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("First Connect() error = %v", err)
	}

	// 第二次连接应该失败
	err = client.Connect(ctx)
	if err == nil {
		t.Error("Second Connect() should return error when already connected")
	}

	client.Disconnect()
}

func TestClient_Disconnect(t *testing.T) {
	server, _, _ := createMockServer(t)
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)

	config := &Config{
		BaseURL:          wsURL,
		Token:            "test-token",
		HandshakeTimeout: 5 * time.Second,
	}

	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	err = client.Disconnect()
	if err != nil {
		t.Errorf("Disconnect() error = %v", err)
	}

	if client.IsConnected() {
		t.Error("Client should be disconnected")
	}

	// 重复断开连接不应该出错
	err = client.Disconnect()
	if err != nil {
		t.Errorf("Second Disconnect() error = %v", err)
	}
}

func TestClient_SendMessage(t *testing.T) {
	server, receivedMessages, _ := createMockServer(t)
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)

	config := &Config{
		BaseURL:          wsURL,
		Token:            "test-token",
		HandshakeTimeout: 5 * time.Second,
	}

	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer client.Disconnect()

	// 发送消息
	content := "Hello, WebSocket!"
	err = client.SendMessage(content)
	if err != nil {
		t.Errorf("SendMessage() error = %v", err)
	}

	// 等待消息被接收
	select {
	case msg := <-receivedMessages:
		if msg.Type != protocol.TypeMessageSend {
			t.Errorf("Message type = %v, want %v", msg.Type, protocol.TypeMessageSend)
		}
		if msg.GetContent() != content {
			t.Errorf("Message content = %v, want %v", msg.GetContent(), content)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for message")
	}
}

func TestClient_SendMessage_NotConnected(t *testing.T) {
	config := &Config{
		BaseURL: "ws://localhost:8080",
		Token:   "test-token",
	}

	client := NewClient(config)

	err := client.SendMessage("Hello")
	if err == nil {
		t.Error("SendMessage() should return error when not connected")
	}
}

func TestClient_SendPing(t *testing.T) {
	server, receivedMessages, _ := createMockServer(t)
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)

	config := &Config{
		BaseURL:          wsURL,
		Token:            "test-token",
		HandshakeTimeout: 5 * time.Second,
	}

	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer client.Disconnect()

	// 发送 ping
	err = client.SendPing()
	if err != nil {
		t.Errorf("SendPing() error = %v", err)
	}

	// 等待 ping 被接收
	select {
	case msg := <-receivedMessages:
		if msg.Type != protocol.TypePing {
			t.Errorf("Message type = %v, want %v", msg.Type, protocol.TypePing)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for ping")
	}
}

func TestClient_ReceiveMessage(t *testing.T) {
	server, _, sendMessages := createMockServer(t)
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)

	config := &Config{
		BaseURL:          wsURL,
		Token:            "test-token",
		HandshakeTimeout: 5 * time.Second,
	}

	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer client.Disconnect()

	// 发送消息到客户端
	testMsg := protocol.NewTextMessage("Test message")
	sendMessages <- testMsg

	// 接收消息
	received, err := client.ReceiveMessage(2 * time.Second)
	if err != nil {
		t.Errorf("ReceiveMessage() error = %v", err)
	}

	if received.GetContent() != "Test message" {
		t.Errorf("Received content = %v, want %v", received.GetContent(), "Test message")
	}
}

func TestClient_ReceiveMessage_Timeout(t *testing.T) {
	server, _, _ := createMockServer(t)
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)

	config := &Config{
		BaseURL:          wsURL,
		Token:            "test-token",
		HandshakeTimeout: 5 * time.Second,
	}

	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer client.Disconnect()

	// 尝试接收消息，应该超时
	_, err = client.ReceiveMessage(100 * time.Millisecond)
	if err == nil {
		t.Error("ReceiveMessage() should return timeout error")
	}
}

func TestClient_SendPicoMessage(t *testing.T) {
	server, receivedMessages, _ := createMockServer(t)
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)

	config := &Config{
		BaseURL:          wsURL,
		Token:            "test-token",
		HandshakeTimeout: 5 * time.Second,
	}

	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer client.Disconnect()

	// 创建自定义 PicoMessage
	msg := protocol.NewMessage(protocol.TypeTypingStart, map[string]any{
		"user_id": "user-123",
	})

	err = client.SendPicoMessage(msg)
	if err != nil {
		t.Errorf("SendPicoMessage() error = %v", err)
	}

	// 等待消息被接收
	select {
	case received := <-receivedMessages:
		if received.Type != protocol.TypeTypingStart {
			t.Errorf("Message type = %v, want %v", received.Type, protocol.TypeTypingStart)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for message")
	}
}
