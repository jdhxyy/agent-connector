package agentconnector

import "time"

type Config struct {
	WebSocket WebSocketConfig
	MQTT      MQTTConfig
	Router    RouterConfig
	AgentID   string
	BufferSize  int
	WorkerCount int
}

type WebSocketConfig struct {
	BaseURL           string
	Token             string
	SessionID         string
	HandshakeTimeout  time.Duration
	PingInterval      time.Duration
	ReconnectMaxRetry int
	ReconnectDelay    time.Duration
}

type MQTTConfig struct {
	BrokerURL            string
	ClientID             string
	Username             string
	Password             string
	KeepAlive            time.Duration
	ConnectTimeout       time.Duration
	CleanSession         bool
	AutoReconnect        bool
	MaxReconnectInterval time.Duration
}

type RouterConfig struct {
	DefaultQoS     byte
	EnableRetry    bool
	MaxRetries     int
	RetryDelay     time.Duration
}

func DefaultConfig() *Config {
	return &Config{
		WebSocket: WebSocketConfig{
			HandshakeTimeout:  10 * time.Second,
			PingInterval:      30 * time.Second,
			ReconnectMaxRetry: 10,
			ReconnectDelay:    3 * time.Second,
		},
		MQTT: MQTTConfig{
			KeepAlive:            60 * time.Second,
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
