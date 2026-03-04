package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/jdhxyy/agent-connector/protocol"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func TestClient_Connect(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		wantError bool
	}{
		{
			name: "successful connection",
			config: &Config{
				BaseURL:          "ws://localhost:8080",
				Token:            "test-token",
				HandshakeTimeout: 5 * time.Second,
			},
			wantError: false,
		},
		{
			name: "invalid URL",
			config: &Config{
				BaseURL:          "://invalid-url",
				Token:            "test-token",
				HandshakeTimeout: 5 * time.Second,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.config)
			assert.NotNil(t, client)
		})
	}
}

func TestClient_SendMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}

			var msg protocol.PicoMessage
			err = msg.FromJSON(data)
			if err != nil {
				continue
			}

			if msg.Type == protocol.TypePing {
				response := protocol.NewMessage(protocol.TypePong, nil)
				respData, _ := response.ToJSON()
				conn.WriteMessage(websocket.TextMessage, respData)
			}
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	config := &Config{
		BaseURL:          wsURL,
		Token:            "test-token",
		HandshakeTimeout: 5 * time.Second,
	}

	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	require.NoError(t, err)
	defer client.Disconnect()

	assert.True(t, client.IsConnected())

	err = client.SendMessage("Hello, World!")
	assert.NoError(t, err)

	err = client.SendPing()
	assert.NoError(t, err)
}

func TestClient_ReceiveMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		msg := protocol.NewTextMessage("Test response")
		data, _ := msg.ToJSON()
		conn.WriteMessage(websocket.TextMessage, data)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	config := &Config{
		BaseURL:          wsURL,
		Token:            "test-token",
		HandshakeTimeout: 5 * time.Second,
	}

	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	require.NoError(t, err)
	defer client.Disconnect()

	msg, err := client.ReceiveMessage(2 * time.Second)
	assert.NoError(t, err)
	assert.Equal(t, "Test response", msg.GetContent())
}

func TestClient_Disconnect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	config := &Config{
		BaseURL:          wsURL,
		Token:            "test-token",
		HandshakeTimeout: 5 * time.Second,
	}

	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, client.IsConnected())

	err = client.Disconnect()
	assert.NoError(t, err)
	assert.False(t, client.IsConnected())

	err = client.Disconnect()
	assert.NoError(t, err)
}

func TestClient_NotConnected(t *testing.T) {
	config := &Config{
		BaseURL:          "ws://localhost:8080",
		Token:            "test-token",
		HandshakeTimeout: 5 * time.Second,
	}

	client := NewClient(config)

	err := client.SendMessage("test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}
