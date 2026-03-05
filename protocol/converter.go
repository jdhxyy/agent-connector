package protocol

import (
	"encoding/json"
	"fmt"
	"time"
)

// Converter 消息协议转换器
// 负责在不同消息格式之间进行转换
type Converter struct{}

// NewConverter 创建新的转换器实例
func NewConverter() *Converter {
	return &Converter{}
}

// PicoToGeneric 将 PicoMessage 转换为 GenericMessage
func (c *Converter) PicoToGeneric(pico PicoMessage, source string) (*GenericMessage, error) {
	payload, err := json.Marshal(pico.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	return &GenericMessage{
		ID:        pico.ID,
		MsgType:   pico.Type,
		Source:    source,
		Payload:   payload,
		Timestamp: time.UnixMilli(pico.Timestamp),
		Metadata: map[string]string{
			"session_id": pico.SessionID,
		},
	}, nil
}

// GenericToPico 将 GenericMessage 转换为 PicoMessage
func (c *Converter) GenericToPico(msg *GenericMessage) (PicoMessage, error) {
	var payload map[string]any
	if len(msg.Payload) > 0 {
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return PicoMessage{}, fmt.Errorf("unmarshal payload: %w", err)
		}
	}

	sessionID := msg.Metadata["session_id"]

	return PicoMessage{
		Type:      msg.MsgType,
		ID:        msg.ID,
		SessionID: sessionID,
		Timestamp: msg.Timestamp.UnixMilli(),
		Payload:   payload,
	}, nil
}

// MQTTToPico 将 MQTT 消息（纯文本）转换为 PicoMessage
func (c *Converter) MQTTToPico(topic string, data []byte) (PicoMessage, error) {
	sessionID := TopicToSessionID(topic)
	if sessionID == "" {
		return PicoMessage{}, fmt.Errorf("invalid topic: %s", topic)
	}

	return PicoMessage{
		Type:      TypeMessageSend,
		Timestamp: time.Now().UnixMilli(),
		SessionID: sessionID,
		Payload: map[string]any{
			"content": string(data),
		},
	}, nil
}

// PicoToMQTT 将 PicoMessage 转换为 MQTT 消息（纯文本）
// 主题生成规则：
//   - agent:{id} -> agent/{id}/message
//   - group:{id} -> group/{id}/message
//   - broadcast:all -> broadcast/message
func (c *Converter) PicoToMQTT(pico PicoMessage) (string, []byte, error) {
	targetType, targetID := ParseSessionID(pico.SessionID)
	if targetID == "" {
		return "", nil, fmt.Errorf("invalid session_id: %s", pico.SessionID)
	}

	var topic string
	switch targetType {
	case "agent":
		topic = fmt.Sprintf("agent/%s/message", targetID)
	case "group":
		topic = fmt.Sprintf("group/%s/message", targetID)
	case "broadcast":
		topic = "broadcast/message"
	default:
		return "", nil, fmt.Errorf("unknown target type: %s", targetType)
	}

	content, ok := pico.Payload["content"].(string)
	if !ok {
		return "", nil, fmt.Errorf("invalid payload content")
	}

	return topic, []byte(content), nil
}

// BuildAgentTopic 构建外部 Agent 的 MQTT Topic
func BuildAgentTopic(agentID string) string {
	return fmt.Sprintf("agent/%s/message", agentID)
}

// BuildGroupTopic 构建外部群组的 MQTT Topic
func BuildGroupTopic(groupID string) string {
	return fmt.Sprintf("group/%s/message", groupID)
}

// BuildBroadcastTopic 构建广播 MQTT Topic
func BuildBroadcastTopic() string {
	return "broadcast/message"
}
