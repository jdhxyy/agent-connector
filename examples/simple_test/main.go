// Agent Connector 示例程序
// 演示如何使用 agent-connector 进行基于新设计文档的消息收发
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	agentconnector "github.com/jdhxyy/agent-connector"
	"github.com/jdhxyy/agent-connector/protocol"
)

func main() {
	var (
		wsURL        = flag.String("ws-url", "ws://localhost:8080", "WebSocket URL")
		wsToken      = flag.String("ws-token", "test-token", "WebSocket Token")
		mqttBroker   = flag.String("mqtt-broker", "tcp://localhost:1883", "MQTT Broker URL")
		mqttClientID = flag.String("mqtt-client-id", "connector-client", "MQTT Client ID")
		mqttUsername = flag.String("mqtt-username", "", "MQTT Username (optional)")
		mqttPassword = flag.String("mqtt-password", "", "MQTT Password (optional)")
		// 外部 Agent ID 列表（逗号分隔）
		externalAgents = flag.String("external-agents", "ext-agent-x,ext-agent-y", "External Agent IDs (comma separated)")
		// 外部群组 ID 列表（逗号分隔）
		externalGroups = flag.String("external-groups", "ext-group-team1", "External Group IDs (comma separated)")
		duration       = flag.Duration("duration", 60*time.Second, "Test duration")
	)
	flag.Parse()

	fmt.Println("=== Agent Connector Test Program (V3 Design) ===")
	fmt.Printf("WebSocket URL: %s\n", *wsURL)
	fmt.Printf("MQTT Broker: %s\n", *mqttBroker)
	fmt.Printf("MQTT Client ID: %s\n", *mqttClientID)
	fmt.Printf("External Agents: %s\n", *externalAgents)
	fmt.Printf("External Groups: %s\n", *externalGroups)
	fmt.Printf("Test Duration: %v\n", *duration)
	fmt.Println("================================================")

	// 创建配置
	config := &agentconnector.Config{
		WebSocket: agentconnector.WebSocketConfig{
			BaseURL:          *wsURL,
			Token:            *wsToken,
			SessionID:        "agent-test",
			HandshakeTimeout: 10 * time.Second,
			PingInterval:     30 * time.Second,
			ConnectTimeout:   10 * time.Second,
		},
		MQTT: agentconnector.MQTTConfig{
			BrokerURL:      *mqttBroker,
			ClientID:       *mqttClientID,
			Username:       *mqttUsername,
			Password:       *mqttPassword,
			KeepAlive:      60,
			ConnectTimeout: 10 * time.Second,
			CleanSession:   true,
		},
		Router: agentconnector.RouterConfig{
			DefaultQoS:  1,
			EnableRetry: true,
			MaxRetries:  3,
			RetryDelay:  1 * time.Second,
		},
	}

	// 创建 connector
	connector, err := agentconnector.NewConnector(config)
	if err != nil {
		log.Fatalf("Failed to create connector: %v", err)
	}

	// 设置消息处理器
	messageCount := 0
	connector.OnMessage(func(msg protocol.Message) error {
		messageCount++
		log.Printf("[Message #%d] Type: %s, Source: %s, Target: %s",
			messageCount,
			msg.GetType(),
			msg.GetSource(),
			msg.GetTarget())
		log.Printf("[Payload] %s", string(msg.GetPayload()))

		if len(msg.GetMetadata()) > 0 {
			log.Printf("[Metadata] %v", msg.GetMetadata())
		}

		return nil
	})

	// 设置错误处理器
	connector.OnError(func(err error, ctx agentconnector.ErrorContext) {
		log.Printf("[Error] Source: %s, Operation: %s, Error: %v",
			ctx.Source,
			ctx.Operation,
			err)
	})

	// 设置状态处理器
	connector.OnStatusChange(func(oldStatus, newStatus agentconnector.ConnectionStatus) {
		if oldStatus != newStatus {
			log.Printf("[Status] %s -> %s", oldStatus.String(), newStatus.String())
		}
	})

	// 启动 connector
	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	log.Println("Starting connector...")
	if err := connector.Start(ctx); err != nil {
		log.Printf("Failed to start connector: %v", err)
		log.Println("Note: This is expected if the WebSocket or MQTT server is not running.")
		log.Println("Please start the servers and try again.")
		os.Exit(1)
	}

	log.Println("Connector started successfully!")

	// 订阅外部 Agents
	if *externalAgents != "" {
		agentIDs := parseCommaSeparated(*externalAgents)
		log.Printf("Subscribing to external agents: %v", agentIDs)
		if err := connector.SubscribeExternalAgents(agentIDs); err != nil {
			log.Printf("Failed to subscribe to external agents: %v", err)
		} else {
			log.Printf("Successfully subscribed to external agents: %v", agentIDs)
		}
	}

	// 订阅外部群组
	if *externalGroups != "" {
		groupIDs := parseCommaSeparated(*externalGroups)
		log.Printf("Subscribing to external groups: %v", groupIDs)
		if err := connector.SubscribeExternalGroups(groupIDs); err != nil {
			log.Printf("Failed to subscribe to external groups: %v", err)
		} else {
			log.Printf("Successfully subscribed to external groups: %v", groupIDs)
		}
	}

	// 获取状态
	status := connector.Status()
	log.Printf("Status: WebSocket=%s, MQTT=%s, Running=%v",
		status.WebSocketStatus.String(),
		status.MQTTStatus.String(),
		status.IsRunning)

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 创建定时发送消息的 ticker（模拟从本地 Agent 发送消息）
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	msgCounter := 0
	testRunning := true

	go func() {
		for testRunning {
			select {
			case <-ticker.C:
				msgCounter++

				// 示例：发送消息给外部 Agent
				picoMsg := protocol.PicoMessage{
					Type:      protocol.TypeMessageSend,
					SessionID: "agent:ext-agent-x",
					Payload: map[string]any{
						"content": fmt.Sprintf("Hello from local agent! Message #%d at %s", msgCounter, time.Now().Format("15:04:05")),
					},
				}

				log.Printf("[Sending] Message #%d to external agent", msgCounter)

				// 通过 WebSocket 发送（模拟本地 Agent 发送）
				// 实际场景中，这是由本地 Agent 通过 WebSocket 发送的
				// 这里仅作演示
				_ = picoMsg

			case <-ctx.Done():
				testRunning = false
				return
			}
		}
	}()

	// 等待信号或超时
	select {
	case sig := <-sigChan:
		log.Printf("Received signal: %v", sig)
	case <-ctx.Done():
		log.Println("Test duration completed")
	}

	testRunning = false

	// 停止 connector
	log.Println("Stopping connector...")
	if err := connector.Stop(); err != nil {
		log.Printf("Error stopping connector: %v", err)
	}

	// 打印统计信息
	finalStatus := connector.Status()
	log.Println("\n=== Test Summary ===")
	log.Printf("Total messages received: %d", messageCount)
	log.Printf("Total messages sent: %d", msgCounter)
	log.Printf("Final status: WebSocket=%s, MQTT=%s",
		finalStatus.WebSocketStatus.String(),
		finalStatus.MQTTStatus.String())
	if !finalStatus.StartTime.IsZero() {
		log.Printf("Uptime: %v", time.Since(finalStatus.StartTime))
	}
	log.Println("===================")

	log.Println("Test completed successfully!")
}

// parseCommaSeparated 解析逗号分隔的字符串
func parseCommaSeparated(s string) []string {
	var result []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			item := s[start:i]
			// 去除空格
			item = trimSpace(item)
			if item != "" {
				result = append(result, item)
			}
			start = i + 1
		}
	}
	return result
}

// trimSpace 去除字符串两端的空格
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
