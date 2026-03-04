package protocol

import (
	"encoding/json"
	"time"
)

const (
	TypeMessageSend   = "message.send"
	TypeMediaSend     = "media.send"
	TypePing          = "ping"
	TypeMessageCreate = "message.create"
	TypeMessageUpdate = "message.update"
	TypeMediaCreate   = "media.create"
	TypeTypingStart   = "typing.start"
	TypeTypingStop    = "typing.stop"
	TypeError         = "error"
	TypePong          = "pong"
)

type PicoMessage struct {
	Type      string         `json:"type"`
	ID        string         `json:"id,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
	Timestamp int64          `json:"timestamp,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
}

func NewMessage(msgType string, payload map[string]any) PicoMessage {
	return PicoMessage{
		Type:      msgType,
		Timestamp: time.Now().UnixMilli(),
		Payload:   payload,
	}
}

func NewTextMessage(content string) PicoMessage {
	return NewMessage(TypeMessageSend, map[string]any{
		"content": content,
	})
}

func NewPing() PicoMessage {
	return NewMessage(TypePing, nil)
}

func (m PicoMessage) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

func (m *PicoMessage) FromJSON(data []byte) error {
	return json.Unmarshal(data, m)
}

func (m PicoMessage) IsError() bool {
	return m.Type == TypeError
}

func (m PicoMessage) GetContent() string {
	if m.Payload == nil {
		return ""
	}
	if content, ok := m.Payload["content"].(string); ok {
		return content
	}
	return ""
}

type Message interface {
	GetID() string
	GetType() string
	GetSource() string
	GetTarget() string
	GetPayload() []byte
	GetTimestamp() time.Time
	GetMetadata() map[string]string
}

type GenericMessage struct {
	ID        string
	MsgType   string
	Source    string
	Target    string
	Payload   []byte
	Timestamp time.Time
	Metadata  map[string]string
}

func (m *GenericMessage) GetID() string              { return m.ID }
func (m *GenericMessage) GetType() string            { return m.MsgType }
func (m *GenericMessage) GetSource() string          { return m.Source }
func (m *GenericMessage) GetTarget() string          { return m.Target }
func (m *GenericMessage) GetPayload() []byte         { return m.Payload }
func (m *GenericMessage) GetTimestamp() time.Time    { return m.Timestamp }
func (m *GenericMessage) GetMetadata() map[string]string { return m.Metadata }
