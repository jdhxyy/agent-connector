package agentconnector

import "time"

// Config 连接器配置结构体
type Config struct {
	WebSocket   WebSocketConfig // WebSocket 连接配置
	MQTT        MQTTConfig      // MQTT 连接配置
	Router      RouterConfig    // 路由器配置
	BufferSize  int             // 消息缓冲区大小（默认 100）
	WorkerCount int             // 工作线程数量（默认 10）
}

// WebSocketConfig WebSocket 连接配置
type WebSocketConfig struct {
	BaseURL           string        // WebSocket 服务器地址
	Token             string        // 认证令牌
	SessionID         string        // 会话 ID（可选，自动生成）
	HandshakeTimeout  time.Duration // 握手超时（默认 10 秒）
	PingInterval      time.Duration // 心跳间隔（默认 30 秒）
	ReconnectMaxRetry int           // 最大重连次数（默认 10）
	ReconnectDelay    time.Duration // 重连延迟（默认 3 秒）
}

// MQTTConfig MQTT 连接配置
type MQTTConfig struct {
	BrokerURL            string        // MQTT Broker 地址
	ClientID             string        // 客户端 ID
	Username             string        // 用户名（可选）
	Password             string        // 密码（可选）
	KeepAlive            uint16        // 保活间隔（秒，默认 60）
	ConnectTimeout       time.Duration // 连接超时（默认 10 秒）
	CleanSession         bool          // 清理会话（默认 false）
	AutoReconnect        bool          // 自动重连（默认 true）
	MaxReconnectInterval time.Duration // 最大重连间隔（默认 10 分钟）
}

// RouterConfig 消息路由器配置
type RouterConfig struct {
	DefaultQoS  byte          // 默认 QoS 等级（默认 1）
	EnableRetry bool          // 启用重试（默认 true）
	MaxRetries  int           // 最大重试次数（默认 3）
	RetryDelay  time.Duration // 重试延迟（默认 1 秒）
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		WebSocket: WebSocketConfig{
			HandshakeTimeout:  10 * time.Second,
			PingInterval:      30 * time.Second,
			ReconnectMaxRetry: 10,
			ReconnectDelay:    3 * time.Second,
		},
		MQTT: MQTTConfig{
			KeepAlive:            60,
			ConnectTimeout:       10 * time.Second,
			CleanSession:         false,
			AutoReconnect:        true,
			MaxReconnectInterval: 10 * time.Minute,
		},
		Router: RouterConfig{
			DefaultQoS:  1,
			EnableRetry: true,
			MaxRetries:  3,
			RetryDelay:  1 * time.Second,
		},
		BufferSize:  100,
		WorkerCount: 10,
	}
}
