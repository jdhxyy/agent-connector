package agentconnector

import (
	"context"
	"testing"
	"time"

	"github.com/jdhxyy/agent-connector/protocol"
)

func TestNewConnector(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errType error
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
			errType: ErrInvalidConfig,
		},
		{
			name: "valid config",
			config: &Config{
				WebSocket: WebSocketConfig{
					BaseURL: "ws://localhost:8080",
					Token:   "test-token",
				},
				MQTT: MQTTConfig{
					BrokerURL: "tcp://localhost:1883",
					ClientID:  "test-client",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := NewConnector(tt.config)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewConnector() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errType != nil && err != tt.errType {
					t.Errorf("NewConnector() error = %v, want %v", err, tt.errType)
				}
			} else {
				if err != nil {
					t.Errorf("NewConnector() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if conn == nil {
					t.Error("NewConnector() returned nil connector")
				}
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	if config.WebSocket.HandshakeTimeout != 10*time.Second {
		t.Errorf("HandshakeTimeout = %v, want %v", config.WebSocket.HandshakeTimeout, 10*time.Second)
	}

	if config.WebSocket.PingInterval != 30*time.Second {
		t.Errorf("PingInterval = %v, want %v", config.WebSocket.PingInterval, 30*time.Second)
	}

	if config.WebSocket.ReconnectMaxRetry != 10 {
		t.Errorf("ReconnectMaxRetry = %v, want %v", config.WebSocket.ReconnectMaxRetry, 10)
	}

	if config.MQTT.KeepAlive != 60 {
		t.Errorf("KeepAlive = %v, want %v", config.MQTT.KeepAlive, 60)
	}

	if config.Router.DefaultQoS != 1 {
		t.Errorf("DefaultQoS = %v, want %v", config.Router.DefaultQoS, 1)
	}

	if config.BufferSize != 100 {
		t.Errorf("BufferSize = %v, want %v", config.BufferSize, 100)
	}
}

func TestConnectorStatus(t *testing.T) {
	config := &Config{
		WebSocket: WebSocketConfig{
			BaseURL: "ws://localhost:8080",
			Token:   "test-token",
		},
		MQTT: MQTTConfig{
			BrokerURL: "tcp://localhost:1883",
			ClientID:  "test-client",
		},
	}

	conn, err := NewConnector(config)
	if err != nil {
		t.Fatalf("NewConnector() error = %v", err)
	}

	status := conn.Status()
	if status.IsRunning {
		t.Error("Initial status should not be running")
	}
	if status.WebSocketStatus != StatusDisconnected {
		t.Errorf("Initial WebSocketStatus = %v, want %v", status.WebSocketStatus, StatusDisconnected)
	}
	if status.MQTTStatus != StatusDisconnected {
		t.Errorf("Initial MQTTStatus = %v, want %v", status.MQTTStatus, StatusDisconnected)
	}
}

func TestConnectionStatus_String(t *testing.T) {
	tests := []struct {
		status ConnectionStatus
		want   string
	}{
		{StatusDisconnected, "disconnected"},
		{StatusConnecting, "connecting"},
		{StatusConnected, "connected"},
		{StatusReconnecting, "reconnecting"},
		{StatusError, "error"},
		{ConnectionStatus(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("ConnectionStatus.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConnectorHandlers(t *testing.T) {
	config := &Config{
		WebSocket: WebSocketConfig{
			BaseURL: "ws://localhost:8080",
			Token:   "test-token",
		},
		MQTT: MQTTConfig{
			BrokerURL: "tcp://localhost:1883",
			ClientID:  "test-client",
		},
	}

	conn, err := NewConnector(config)
	if err != nil {
		t.Fatalf("NewConnector() error = %v", err)
	}

	conn.OnMessage(func(msg protocol.Message) error {
		t.Log("Message received")
		return nil
	})

	conn.OnError(func(err error, ctx ErrorContext) {
		t.Logf("Error received: %v", err)
	})

	conn.OnStatusChange(func(oldStatus, newStatus ConnectionStatus) {
		t.Logf("Status changed: %s -> %s", oldStatus.String(), newStatus.String())
	})

	t.Log("Handlers registered successfully")
}

func TestDefaultReconnectPolicy(t *testing.T) {
	policy := DefaultReconnectPolicy()

	if policy == nil {
		t.Fatal("DefaultReconnectPolicy() returned nil")
	}

	if policy.MaxRetries != 10 {
		t.Errorf("MaxRetries = %v, want %v", policy.MaxRetries, 10)
	}

	if policy.InitialDelay != 1*time.Second {
		t.Errorf("InitialDelay = %v, want %v", policy.InitialDelay, 1*time.Second)
	}

	if policy.MaxDelay != 30*time.Second {
		t.Errorf("MaxDelay = %v, want %v", policy.MaxDelay, 30*time.Second)
	}

	if policy.BackoffMultiplier != 2.0 {
		t.Errorf("BackoffMultiplier = %v, want %v", policy.BackoffMultiplier, 2.0)
	}
}

func TestConnectorIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := &Config{
		WebSocket: WebSocketConfig{
			BaseURL:          "ws://localhost:8080",
			Token:            "test-token",
			HandshakeTimeout: 5 * time.Second,
			PingInterval:     10 * time.Second,
		},
		MQTT: MQTTConfig{
			BrokerURL:      "tcp://localhost:1883",
			ClientID:       "test-client",
			ConnectTimeout: 5 * time.Second,
		},
		Router: RouterConfig{
			DefaultQoS: 1,
		},
	}

	conn, err := NewConnector(config)
	if err != nil {
		t.Fatalf("NewConnector() error = %v", err)
	}

	messageCount := 0
	conn.OnMessage(func(msg protocol.Message) error {
		messageCount++
		t.Logf("Received message: %s", string(msg.GetPayload()))
		return nil
	})

	conn.OnError(func(err error, ctx ErrorContext) {
		t.Logf("Error: %v, Context: %+v", err, ctx)
	})

	conn.OnStatusChange(func(oldStatus, newStatus ConnectionStatus) {
		t.Logf("Status changed: %s -> %s", oldStatus.String(), newStatus.String())
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = conn.Start(ctx)
	if err != nil {
		t.Logf("Start() error (expected if no server): %v", err)
		return
	}

	defer conn.Stop()

	status := conn.Status()
	if !status.IsRunning {
		t.Error("Connector should be running after Start()")
	}

	t.Logf("Connector started successfully. Status: %+v", status)
}
