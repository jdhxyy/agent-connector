package agentconnector

import (
	"context"
	"time"

	"github.com/jdhxyy/agent-connector/protocol"
)

// ConnectionStatus 连接状态枚举
type ConnectionStatus int

const (
	StatusDisconnected ConnectionStatus = iota // 断开连接
	StatusConnecting                           // 连接中
	StatusConnected                            // 已连接
	StatusReconnecting                         // 重连中
	StatusError                                // 错误状态
)

// String 返回连接状态的字符串表示
func (s ConnectionStatus) String() string {
	switch s {
	case StatusDisconnected:
		return "disconnected"
	case StatusConnecting:
		return "connecting"
	case StatusConnected:
		return "connected"
	case StatusReconnecting:
		return "reconnecting"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

// MessageHandler 消息处理函数类型
type MessageHandler func(msg protocol.Message) error

// ErrorHandler 错误处理函数类型
type ErrorHandler func(err error, context ErrorContext)

// StatusHandler 状态变更处理函数类型
type StatusHandler func(oldStatus, newStatus ConnectionStatus)

// ConnectionLostHandler 连接丢失处理函数类型
type ConnectionLostHandler func(clientID string, err error)

// ReconnectingHandler 重连处理函数类型
type ReconnectingHandler func(clientID string)

// OnConnectHandler 连接成功处理函数类型
type OnConnectHandler func(clientID string)

// ReconnectPolicy 重连策略配置
type ReconnectPolicy struct {
	MaxRetries        int
	InitialDelay      time.Duration
	MaxDelay          time.Duration
	BackoffMultiplier float64
}

// DefaultReconnectPolicy 返回默认重连策略
func DefaultReconnectPolicy() *ReconnectPolicy {
	return &ReconnectPolicy{
		MaxRetries:        10,
		InitialDelay:      1 * time.Second,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
	}
}

// ConnectorStatus 连接器整体状态
type ConnectorStatus struct {
	WebSocketStatus ConnectionStatus
	MQTTStatus      ConnectionStatus
	IsRunning       bool
	StartTime       time.Time
}

// Connector 连接器接口
type Connector interface {
	Start(ctx context.Context) error
	Stop() error
	Status() ConnectorStatus

	SubscribeExternalAgents(agentIDs []string) error
	SubscribeExternalGroups(groupIDs []string) error
	UnsubscribeExternalAgents(agentIDs []string) error
	UnsubscribeExternalGroups(groupIDs []string) error

	OnMessage(handler MessageHandler)
	OnError(handler ErrorHandler)
	OnStatusChange(handler StatusHandler)
}
