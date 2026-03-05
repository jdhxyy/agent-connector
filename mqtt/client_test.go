package mqtt

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	config := &Config{
		BrokerURL:            "tcp://localhost:1883",
		ClientID:             "test-client",
		Username:             "test-user",
		Password:             "test-pass",
		KeepAlive:            60,
		ConnectTimeout:       10 * time.Second,
		CleanSession:         false,
		AutoReconnect:        true,
		MaxReconnectInterval: 10 * time.Minute,
	}

	client := NewClient(config)
	if client == nil {
		t.Fatal("NewClient() returned nil")
	}

	if client.GetStatus() != StatusDisconnected {
		t.Errorf("Initial status = %v, want %v", client.GetStatus(), StatusDisconnected)
	}

	if client.IsConnected() {
		t.Error("Client should not be connected initially")
	}
}

func TestClient_SetHandlers(t *testing.T) {
	config := &Config{
		BrokerURL: "tcp://localhost:1883",
		ClientID:  "test-client",
	}

	client := NewClient(config)

	// 设置连接丢失处理器
	client.SetConnectionLostHandler(func(clientID string, err error) {
		t.Logf("Connection lost: clientID=%s, err=%v", clientID, err)
	})

	// 设置重连处理器
	client.SetReconnectingHandler(func(clientID string) {
		t.Logf("Reconnecting: clientID=%s", clientID)
	})

	// 设置连接成功处理器
	client.SetOnConnectHandler(func(clientID string) {
		t.Logf("Connected: clientID=%s", clientID)
	})

	// 验证处理器已设置（这里只是验证不会 panic）
	t.Log("Handlers set successfully")
}

func TestClient_Connect_NotRunningBroker(t *testing.T) {
	// 使用一个不会响应的地址
	config := &Config{
		BrokerURL:      "tcp://localhost:19999",
		ClientID:       "test-client",
		ConnectTimeout: 1 * time.Second,
	}

	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 应该超时或连接失败
	err := client.Connect(ctx)
	if err == nil {
		t.Error("Connect() should return error when broker is not running")
	}

	if client.IsConnected() {
		t.Error("Client should not be connected after failed connection")
	}
}

func TestClient_Subscribe_NotConnected(t *testing.T) {
	config := &Config{
		BrokerURL: "tcp://localhost:1883",
		ClientID:  "test-client",
	}

	client := NewClient(config)

	err := client.Subscribe("test/topic", 1, func(topic string, data []byte) {
		t.Logf("Received on %s: %s", topic, string(data))
	})

	if err == nil {
		t.Error("Subscribe() should return error when not connected")
	}
}

func TestClient_Unsubscribe_NotConnected(t *testing.T) {
	config := &Config{
		BrokerURL: "tcp://localhost:1883",
		ClientID:  "test-client",
	}

	client := NewClient(config)

	err := client.Unsubscribe("test/topic")
	if err == nil {
		t.Error("Unsubscribe() should return error when not connected")
	}
}

func TestClient_Publish_NotConnected(t *testing.T) {
	config := &Config{
		BrokerURL: "tcp://localhost:1883",
		ClientID:  "test-client",
	}

	client := NewClient(config)

	err := client.Publish("test/topic", 1, false, "test message")
	if err == nil {
		t.Error("Publish() should return error when not connected")
	}
}

func TestClient_Disconnect_NotConnected(t *testing.T) {
	config := &Config{
		BrokerURL: "tcp://localhost:1883",
		ClientID:  "test-client",
	}

	client := NewClient(config)

	// 断开未连接的客户端不应该出错
	err := client.Disconnect(1 * time.Second)
	if err != nil {
		t.Errorf("Disconnect() error = %v", err)
	}
}

func TestClient_SubscribeMultiple_NotConnected(t *testing.T) {
	config := &Config{
		BrokerURL: "tcp://localhost:1883",
		ClientID:  "test-client",
	}

	client := NewClient(config)

	topics := map[string]byte{
		"test/topic1": 1,
		"test/topic2": 1,
	}

	err := client.SubscribeMultiple(topics, func(topic string, data []byte) {
		t.Logf("Received on %s: %s", topic, string(data))
	})

	if err == nil {
		t.Error("SubscribeMultiple() should return error when not connected")
	}
}

// 集成测试 - 需要运行中的 MQTT Broker
func TestClientIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := &Config{
		BrokerURL:            "tcp://localhost:1883",
		ClientID:             "integration-test-client",
		KeepAlive:            30,
		ConnectTimeout:       5 * time.Second,
		CleanSession:         true,
		AutoReconnect:        true,
		MaxReconnectInterval: 1 * time.Minute,
	}

	client := NewClient(config)

	// 设置处理器
	client.SetConnectionLostHandler(func(clientID string, err error) {
		t.Logf("Connection lost: %s, %v", clientID, err)
	})

	client.SetOnConnectHandler(func(clientID string) {
		t.Logf("Connected: %s", clientID)
	})

	// 连接
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	if err != nil {
		t.Skipf("Cannot connect to MQTT broker: %v (is broker running?)", err)
	}

	if !client.IsConnected() {
		t.Error("Client should be connected")
	}

	defer client.Disconnect(5 * time.Second)

	// 测试订阅
	messageReceived := make(chan struct {
		topic string
		data  []byte
	}, 1)

	err = client.Subscribe("test/integration", 1, func(topic string, data []byte) {
		messageReceived <- struct {
			topic string
			data  []byte
		}{topic: topic, data: data}
	})
	if err != nil {
		t.Errorf("Subscribe() error = %v", err)
	}

	// 测试发布
	testPayload := "Hello MQTT!"
	err = client.Publish("test/integration", 1, false, testPayload)
	if err != nil {
		t.Errorf("Publish() error = %v", err)
	}

	// 等待接收消息
	select {
	case msg := <-messageReceived:
		t.Logf("Received message on %s: %s", msg.topic, string(msg.data))
		if string(msg.data) != testPayload {
			t.Errorf("Payload mismatch: got %s, want %s", string(msg.data), testPayload)
		}
	case <-time.After(5 * time.Second):
		t.Error("Timeout waiting for message")
	}

	// 测试取消订阅
	err = client.Unsubscribe("test/integration")
	if err != nil {
		t.Errorf("Unsubscribe() error = %v", err)
	}

	t.Log("Integration test completed successfully")
}

// 测试多个主题订阅
func TestClientIntegration_MultipleSubscriptions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := &Config{
		BrokerURL:      "tcp://localhost:1883",
		ClientID:       "integration-test-client-multi",
		ConnectTimeout: 5 * time.Second,
		CleanSession:   true,
	}

	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	if err != nil {
		t.Skipf("Cannot connect to MQTT broker: %v (is broker running?)", err)
	}
	defer client.Disconnect(5 * time.Second)

	// 订阅多个主题
	topics := map[string]byte{
		"test/topic1": 1,
		"test/topic2": 1,
	}

	receivedCount := 0
	err = client.SubscribeMultiple(topics, func(topic string, data []byte) {
		receivedCount++
		t.Logf("Received on %s: %s", topic, string(data))
	})
	if err != nil {
		t.Errorf("SubscribeMultiple() error = %v", err)
	}

	// 向每个主题发布消息
	for topic := range topics {
		err = client.Publish(topic, 1, false, "Hello")
		if err != nil {
			t.Errorf("Publish() to %s error = %v", topic, err)
		}
	}

	// 等待消息
	time.Sleep(2 * time.Second)

	t.Logf("Received %d messages", receivedCount)
}

// 测试 QoS 级别
func TestClientIntegration_DifferentQoS(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := &Config{
		BrokerURL:      "tcp://localhost:1883",
		ClientID:       "integration-test-client-qos",
		ConnectTimeout: 5 * time.Second,
		CleanSession:   true,
	}

	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	if err != nil {
		t.Skipf("Cannot connect to MQTT broker: %v (is broker running?)", err)
	}
	defer client.Disconnect(5 * time.Second)

	// 测试不同 QoS 级别
	qosLevels := []byte{0, 1, 2}

	for _, qos := range qosLevels {
		topic := fmt.Sprintf("test/qos/%d", qos)
		q := qos // 捕获循环变量
		err := client.Subscribe(topic, qos, func(rcvTopic string, data []byte) {
			t.Logf("Received QoS %d message on %s: %s", q, rcvTopic, string(data))
		})
		if err != nil {
			t.Errorf("Subscribe() with QoS %d error = %v", qos, err)
		}

		err = client.Publish(topic, qos, false, fmt.Sprintf("QoS %d test", qos))
		if err != nil {
			t.Errorf("Publish() with QoS %d error = %v", qos, err)
		}
	}

	time.Sleep(2 * time.Second)
}
