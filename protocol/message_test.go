package protocol

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewMessage(t *testing.T) {
	payload := map[string]any{"key": "value"}
	msg := NewMessage(TypeMessageSend, payload)

	assert.Equal(t, TypeMessageSend, msg.Type)
	assert.Equal(t, payload, msg.Payload)
	assert.NotZero(t, msg.Timestamp)
}

func TestNewTextMessage(t *testing.T) {
	content := "Hello, World!"
	msg := NewTextMessage(content)

	assert.Equal(t, TypeMessageSend, msg.Type)
	assert.Equal(t, content, msg.GetContent())
}

func TestNewPing(t *testing.T) {
	msg := NewPing()

	assert.Equal(t, TypePing, msg.Type)
	assert.Nil(t, msg.Payload)
}

func TestPicoMessage_ToJSON(t *testing.T) {
	msg := PicoMessage{
		Type:      TypeMessageSend,
		ID:        "msg-123",
		SessionID: "session-456",
		Timestamp: time.Now().UnixMilli(),
		Payload: map[string]any{
			"content": "test message",
		},
	}

	data, err := msg.ToJSON()
	assert.NoError(t, err)
	assert.NotNil(t, data)
	assert.Contains(t, string(data), "message.send")
	assert.Contains(t, string(data), "msg-123")
}

func TestPicoMessage_FromJSON(t *testing.T) {
	jsonData := `{"type":"message.send","id":"msg-123","session_id":"session-456","timestamp":1700000000000,"payload":{"content":"test"}}`

	var msg PicoMessage
	err := msg.FromJSON([]byte(jsonData))
	assert.NoError(t, err)
	assert.Equal(t, TypeMessageSend, msg.Type)
	assert.Equal(t, "msg-123", msg.ID)
	assert.Equal(t, "session-456", msg.SessionID)
	assert.Equal(t, int64(1700000000000), msg.Timestamp)
	assert.Equal(t, "test", msg.GetContent())
}

func TestPicoMessage_IsError(t *testing.T) {
	errorMsg := PicoMessage{Type: TypeError}
	normalMsg := PicoMessage{Type: TypeMessageSend}

	assert.True(t, errorMsg.IsError())
	assert.False(t, normalMsg.IsError())
}

func TestPicoMessage_GetContent(t *testing.T) {
	tests := []struct {
		name     string
		payload  map[string]any
		expected string
	}{
		{
			name:     "with content",
			payload:  map[string]any{"content": "hello"},
			expected: "hello",
		},
		{
			name:     "without content",
			payload:  map[string]any{"other": "value"},
			expected: "",
		},
		{
			name:     "nil payload",
			payload:  nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := PicoMessage{Payload: tt.payload}
			assert.Equal(t, tt.expected, msg.GetContent())
		})
	}
}

func TestGenericMessage(t *testing.T) {
	msg := &GenericMessage{
		ID:        "msg-1",
		MsgType:   "text",
		Source:    "agent-a",
		Target:    "agent-b",
		Payload:   []byte("hello"),
		Timestamp: time.Now(),
		Metadata:  map[string]string{"key": "value"},
	}

	assert.Equal(t, "msg-1", msg.GetID())
	assert.Equal(t, "text", msg.GetType())
	assert.Equal(t, "agent-a", msg.GetSource())
	assert.Equal(t, "agent-b", msg.GetTarget())
	assert.Equal(t, []byte("hello"), msg.GetPayload())
	assert.NotZero(t, msg.GetTimestamp())
	assert.Equal(t, map[string]string{"key": "value"}, msg.GetMetadata())
}
