package protocol

import (
	"encoding/json"
	"time"
)

// 消息类型常量
const (
	TypeMessageSend   = "message.send"   // 文本消息发送
	TypeMediaSend     = "media.send"     // 媒体消息发送
	TypePing          = "ping"           // 心跳请求
	TypeMessageCreate = "message.create" // 消息创建
	TypeMessageUpdate = "message.update" // 消息更新
	TypeMediaCreate   = "media.create"   // 媒体创建
	TypeTypingStart   = "typing.start"   // 开始输入
	TypeTypingStop    = "typing.stop"    // 结束输入
	TypeError         = "error"          // 错误消息
	TypePong          = "pong"           // 心跳响应
)

// PicoMessage Pico 协议消息结构体
// 用于 WebSocket 通信的消息格式
//
// 字段说明：
//   - Type: 消息类型，如 "message.send"、"ping" 等
//   - ID: 消息唯一标识符
//   - SessionID: 会话 ID，格式为 "agent:{id}"、"group:{id}" 或 "broadcast:all"
//   - Timestamp: 时间戳（毫秒）
//   - Payload: 消息负载，内容为键值对
type PicoMessage struct {
	Type      string         `json:"type"`                 // 消息类型
	ID        string         `json:"id,omitempty"`         // 消息 ID（可选）
	SessionID string         `json:"session_id,omitempty"` // 会话 ID（可选）
	Timestamp int64          `json:"timestamp,omitempty"`  // 时间戳（可选）
	Payload   map[string]any `json:"payload,omitempty"`    // 消息负载（可选）
}

// NewMessage 创建新的 PicoMessage
func NewMessage(msgType string, payload map[string]any) PicoMessage {
	return PicoMessage{
		Type:      msgType,
		Timestamp: time.Now().UnixMilli(),
		Payload:   payload,
	}
}

// NewTextMessage 创建文本消息
func NewTextMessage(content string) PicoMessage {
	return NewMessage(TypeMessageSend, map[string]any{
		"content": content,
	})
}

// NewPing 创建心跳 ping 消息
func NewPing() PicoMessage {
	return NewMessage(TypePing, nil)
}

// ToJSON 将 PicoMessage 序列化为 JSON 字节切片
func (m PicoMessage) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// FromJSON 从 JSON 字节切片反序列化 PicoMessage
func (m *PicoMessage) FromJSON(data []byte) error {
	return json.Unmarshal(data, m)
}

// IsError 判断是否为错误消息
func (m PicoMessage) IsError() bool {
	return m.Type == TypeError
}

// GetContent 获取消息文本内容
func (m PicoMessage) GetContent() string {
	if m.Payload == nil {
		return ""
	}
	if content, ok := m.Payload["content"].(string); ok {
		return content
	}
	return ""
}

// Message 通用消息接口
type Message interface {
	GetID() string
	GetType() string
	GetSource() string
	GetTarget() string
	GetPayload() []byte
	GetTimestamp() time.Time
	GetMetadata() map[string]string
}

// GenericMessage 通用消息结构体
// 用于系统内部和 MQTT 通信的消息格式
//
// 字段说明：
//   - ID: 消息唯一标识符
//   - MsgType: 消息类型
//   - Source: 消息来源 Agent ID
//   - Target: 消息目标 Agent ID
//   - Payload: 消息负载（字节数组）
//   - Timestamp: 时间戳
//   - Metadata: 元数据（键值对）
type GenericMessage struct {
	ID        string            // 消息 ID
	MsgType   string            // 消息类型
	Source    string            // 来源 Agent
	Target    string            // 目标 Agent
	Payload   []byte            // 消息负载
	Timestamp time.Time         // 时间戳
	Metadata  map[string]string // 元数据
}

func (m *GenericMessage) GetID() string                  { return m.ID }
func (m *GenericMessage) GetType() string                { return m.MsgType }
func (m *GenericMessage) GetSource() string              { return m.Source }
func (m *GenericMessage) GetTarget() string              { return m.Target }
func (m *GenericMessage) GetPayload() []byte             { return m.Payload }
func (m *GenericMessage) GetTimestamp() time.Time        { return m.Timestamp }
func (m *GenericMessage) GetMetadata() map[string]string { return m.Metadata }

// MessageHandler 消息处理函数类型r func(Message) errorr func(Message) error
type MessageHandler func(Message) error
