package agentconnector

import "errors"

var (
	ErrConnectionFailed     = errors.New("connection failed")
	ErrSendTimeout          = errors.New("send timeout")
	ErrInvalidMessage       = errors.New("invalid message")
	ErrRouteNotFound        = errors.New("route not found")
	ErrAuthentication       = errors.New("authentication failed")
	ErrNotConnected         = errors.New("not connected")
	ErrAlreadyConnected     = errors.New("already connected")
	ErrInvalidConfig        = errors.New("invalid configuration")
	ErrSubscriptionFailed   = errors.New("subscription failed")
	ErrPublishFailed        = errors.New("publish failed")
	ErrClientDisconnected   = errors.New("client disconnected")
)

type ErrorContext struct {
	Source    string
	Operation string
	MessageID string
	Timestamp int64
}
