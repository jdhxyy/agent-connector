package protocol

import "strings"

// TopicToSessionID 从 MQTT Topic 生成 SessionID
// 示例:
//   - "agent/ext-agent-x/message" -> "agent:ext-agent-x"
//   - "group/ext-group-team1/message" -> "group:ext-group-team1"
//   - "broadcast/message" -> "broadcast:all"
func TopicToSessionID(topic string) string {
	parts := strings.Split(topic, "/")
	if len(parts) < 2 {
		return ""
	}

	switch parts[0] {
	case "agent":
		if len(parts) >= 2 {
			return "agent:" + parts[1]
		}
	case "group":
		if len(parts) >= 2 {
			return "group:" + parts[1]
		}
	case "broadcast":
		return "broadcast:all"
	}
	return ""
}

// ParseSessionID 解析 SessionID
// 示例:
//   - "agent:ext-agent-x" -> ("agent", "ext-agent-x")
//   - "group:ext-group-team1" -> ("group", "ext-group-team1")
func ParseSessionID(sessionID string) (targetType, targetID string) {
	parts := strings.SplitN(sessionID, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

// NewSessionID 创建 SessionID
func NewSessionID(targetType, targetID string) string {
	return targetType + ":" + targetID
}

// GetTargetType 获取 SessionID 的目标类型
func GetTargetType(sessionID string) string {
	targetType, _ := ParseSessionID(sessionID)
	return targetType
}

// GetTargetID 获取 SessionID 的目标 ID
func GetTargetID(sessionID string) string {
	_, targetID := ParseSessionID(sessionID)
	return targetID
}

// IsAgentSession 判断是否为 Agent 类型的 SessionID
func IsAgentSession(sessionID string) bool {
	targetType, _ := ParseSessionID(sessionID)
	return targetType == "agent"
}

// IsGroupSession 判断是否为 Group 类型的 SessionID
func IsGroupSession(sessionID string) bool {
	targetType, _ := ParseSessionID(sessionID)
	return targetType == "group"
}

// IsBroadcastSession 判断是否为 Broadcast 类型的 SessionID
func IsBroadcastSession(sessionID string) bool {
	targetType, _ := ParseSessionID(sessionID)
	return targetType == "broadcast"
}
