package agentconnector

import (
	"context"
	"time"

	"github.com/jdhxyy/agent-connector/protocol"
)

type ConnectionStatus int

const (
	StatusDisconnected ConnectionStatus = iota
	StatusConnecting
	StatusConnected
	StatusReconnecting
	StatusError
)

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

type MessageHandler func(msg protocol.Message) error

type ErrorHandler func(err error, context ErrorContext)

type StatusHandler func(oldStatus, newStatus ConnectionStatus)

type ConnectionLostHandler func(clientID string, err error)

type ReconnectingHandler func(clientID string)

type OnConnectHandler func(clientID string)

type ReconnectPolicy struct {
	MaxRetries        int
	InitialDelay      time.Duration
	MaxDelay          time.Duration
	BackoffMultiplier float64
}

func DefaultReconnectPolicy() *ReconnectPolicy {
	return &ReconnectPolicy{
		MaxRetries:        10,
		InitialDelay:      1 * time.Second,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
	}
}

type Subscription struct {
	Topic     string
	QoS       byte
	CreatedAt time.Time
	Handler   MessageHandler
}

type RouteRule struct {
	Source      string
	SourceAgent string
	TargetAgent string
	MessageType string
	Transform   func(protocol.Message) (protocol.Message, error)
}

type ConnectorStatus struct {
	WebSocketStatus ConnectionStatus
	MQTTStatus      ConnectionStatus
	IsRunning       bool
	StartTime       time.Time
}

type Connector interface {
	Start(ctx context.Context) error
	Stop() error
	Status() ConnectorStatus
	SendToAgent(agentID string, msg protocol.Message) error
	Broadcast(msg protocol.Message) error
	SubscribeAgent(agentID string) error
	UnsubscribeAgent(agentID string) error
	OnMessage(handler MessageHandler)
	OnError(handler ErrorHandler)
	OnStatusChange(handler StatusHandler)
}
