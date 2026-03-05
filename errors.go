package agentconnector

import "errors"

// =============================================================================
// 错误定义
// =============================================================================

// 预定义的错误变量
var (
	// ErrConnectionFailed 连接失败
	// 当无法建立网络连接时返回
	ErrConnectionFailed = errors.New("connection failed")

	// ErrSendTimeout 发送超时
	// 当消息发送超时（未收到确认）时返回
	ErrSendTimeout = errors.New("send timeout")

	// ErrInvalidMessage 无效消息
	// 当消息格式不正确或内容为空时返回
	ErrInvalidMessage = errors.New("invalid message")

	// ErrRouteNotFound 未找到路由
	// 当无法找到消息的目标路由时返回
	ErrRouteNotFound = errors.New("route not found")

	// ErrAuthentication 认证失败
	// 当认证令牌无效或过期时返回
	ErrAuthentication = errors.New("authentication failed")

	// ErrNotConnected 未连接
	// 当尝试操作一个未连接的客户端时返回
	ErrNotConnected = errors.New("not connected")

	// ErrAlreadyConnected 已连接
	// 当尝试连接一个已经连接的客户端时返回
	ErrAlreadyConnected = errors.New("already connected")

	// ErrInvalidConfig 无效配置
	// 当配置参数无效或缺失必需字段时返回
	ErrInvalidConfig = errors.New("invalid configuration")

	// ErrSubscriptionFailed 订阅失败
	// 当订阅主题失败时返回
	ErrSubscriptionFailed = errors.New("subscription failed")

	// ErrPublishFailed 发布失败
	// 当发布消息失败时返回
	ErrPublishFailed = errors.New("publish failed")

	// ErrClientDisconnected 客户端已断开
	// 当操作一个已断开的客户端时返回
	ErrClientDisconnected = errors.New("client disconnected")
)

// =============================================================================
// 错误上下文
// =============================================================================

// ErrorContext 错误上下文信息
// 用于在错误处理中传递详细的错误上下文
//
// 字段说明：
//   - Source: 错误来源，如 "websocket"、"mqtt"、"router" 等
//   - Operation: 发生错误的操作，如 "connect"、"send"、"subscribe" 等
//   - MessageID: 相关的消息 ID（可选）
//   - Timestamp: 错误发生的时间戳（Unix 时间戳）
type ErrorContext struct {
	Source    string // 错误来源模块
	Operation string // 失败的操作名称
	MessageID string // 相关消息 ID
	Timestamp int64  // 时间戳
}
