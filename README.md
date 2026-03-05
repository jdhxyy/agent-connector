# agent-connector

AI 聊天系统的核心通信组件，提供 WebSocket 和 MQTT 双协议支持，实现 Agent 间的消息转发。

## 功能特性

- ✅ WebSocket 客户端（Pico Protocol 兼容）
- ✅ MQTT 客户端（支持 QoS 0/1/2）
- ✅ 消息路由与转发
- ✅ 自动重连机制
- ✅ 连接状态监控
- ✅ 完整的错误处理

## 安装

```bash
go get github.com/jdhxyy/agent-connector
```

## 快速开始

```go
package main

import (
    "context"
    "log"
    "time"
    
    agentconnector "github.com/jdhxyy/agent-connector"
)

func main() {
    config := &agentconnector.Config{
        AgentID: "my-agent",
        WebSocket: agentconnector.WebSocketConfig{
            BaseURL: "ws://localhost:8080",
            Token:   "your-token",
        },
        MQTT: agentconnector.MQTTConfig{
            BrokerURL: "tcp://localhost:1883",
            ClientID:  "my-agent-client",
        },
    }

    connector, err := agentconnector.NewConnector(config)
    if err != nil {
        log.Fatal(err)
    }

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := connector.Start(ctx); err != nil {
        log.Fatal(err)
    }
    defer connector.Stop()

    // 订阅消息
    connector.OnMessage(func(msg agentconnector.Message) error {
        log.Printf("Received: %s", msg.GetPayload())
        return nil
    })

    // 发送消息给外部 MQTT Agent
    connector.SendToExternalAgent("other-agent", msg)
}
```

## 测试

```bash
go test -v ./...
go test -cover ./...
```

## 许可证

MIT
