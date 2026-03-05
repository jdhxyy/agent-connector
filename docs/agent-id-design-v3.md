# Agent 转发器 ID 设计方案 V3（最终版）

## 文档信息

- **版本**: v3.1（最终极简版）
- **日期**: 2026-03-04
- **作者**: AI Assistant
- **状态**: 最终版（采用极简 MQTT 消息格式）

---

## 1. 核心概念定义

### 1.1 架构概念

```
┌─────────────────────────────────────────────────────────────────┐
│                         本机 (Local)                             │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              Agent 转发器 (Connector)                    │   │
│  │  ┌─────────────────────┐  ┌─────────────────────────┐  │   │
│  │  │  WebSocket 客户端    │  │    MQTT 客户端          │  │   │
│  │  │  - 连接本机 Agent    │  │  - 连接外部 Agent        │  │   │
│  │  │  - 一对一连接        │  │  - 连接外部群组          │  │   │
│  │  └──────────┬──────────┘  └──────────┬──────────────┘  │   │
│  │             │                        │                  │   │
│  │             ▼                        ▼                  │   │
│  │     本机 Agent (唯一)         外部 Agent X              │   │
│  │                               外部群组 G                │   │
│  │                               ...                      │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### 1.2 概念区分

| 概念 | 定义 | 连接方式 | 示例 |
|------|------|---------|------|
| **本机 Agent** | 通过 WebSocket 连接到转发器的 Agent（一对一） | WebSocket | `local-user1` |
| **外部 Agent** | 通过 MQTT 连接到转发器的 Agent | MQTT | `ext-agent-x`, `ext-agent-y` |
| **外部群组** | 通过 MQTT 订阅的群组 | MQTT | `ext-group-team1` |

### 1.3 通信方向

| 方向 | 说明 | 示例 |
|------|------|------|
| **本机 → 外部** | 本机 Agent 发送消息给外部 Agent/群组 | 本机 Agent 发送给外部 Agent X |
| **外部 → 本机** | 外部 Agent/群组 发送消息给本机 Agent | 外部 Agent X 发送给本机 Agent |

---

## 2. 消息格式设计

### 2.1 WebSocket 消息格式（PicoMessage）

```json
{
  "type": "message.send",
  "session_id": "agent:ext-agent-x",
  "payload": {
    "content": "Hello External Agent!"
  }
}
```

**字段说明**：
- `type`: 消息类型（message.send, typing.start 等）
- `session_id`: 会话标识，格式为 `{target-type}:{target-id}`
- `payload`: 消息负载，固定格式 `{"content": "..."}`

### 2.2 MQTT 消息格式（极简）

```
纯文本内容，无 JSON 包装

示例: "Hello External Agent!"
```

**设计原则**：
- MQTT 消息直接承载文本内容
- 所有路由信息通过 Topic 传递
- SessionID 由 Topic 自动生成
- 无需额外的元数据包装

---

## 3. SessionID 设计

### 3.1 格式定义

```
SessionID = {target-type}:{target-id}

其中:
- target-type: 目标类型 ("agent" 或 "group")
- target-id: 目标 ID (外部 Agent ID 或外部群组 ID)
```

### 3.2 Topic → SessionID 映射

| MQTT Topic | 生成的 SessionID |
|-----------|-----------------|
| `agent/{external-agent-id}/message` | `agent:{external-agent-id}` |
| `group/{external-group-id}/message` | `group:{external-group-id}` |
| `broadcast/message` | `broadcast:all` |

### 3.3 辅助函数

```go
package protocol

import "strings"

// TopicToSessionID 从 MQTT Topic 生成 SessionID
// 参数:
//   - topic: MQTT Topic
// 返回值:
//   - string: 生成的 SessionID
//
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
        return "agent:" + parts[1]
    case "group":
        return "group:" + parts[1]
    case "broadcast":
        return "broadcast:all"
    }
    return ""
}

// ParseSessionID 解析 SessionID
// 参数:
//   - sessionID: SessionID 字符串
// 返回值:
//   - targetType: 目标类型 ("agent"/"group"/"broadcast")
//   - targetID: 目标 ID
//
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
// 参数:
//   - targetType: 目标类型
//   - targetID: 目标 ID
// 返回值:
//   - string: SessionID
func NewSessionID(targetType, targetID string) string {
    return targetType + ":" + targetID
}
```

---

## 4. MQTT Topic 设计

### 4.1 Topic 层级结构

```
agent/                          # 外部 Agent 消息
└── {external-agent-id}/        # 外部 Agent ID
    └── message                 # 文本消息

group/                          # 外部群组消息
└── {external-group-id}/        # 外部群组 ID
    └── message                 # 群聊消息

broadcast/                      # 广播消息
└── message                     # 广播文本
```

### 4.2 Topic 命名规范

| 场景 | Topic 格式 | 示例 | 说明 |
|------|-----------|------|------|
| 外部 Agent 消息 | `agent/{external-agent-id}/message` | `agent/ext-agent-x/message` | 发给外部 Agent |
| 外部群组消息 | `group/{external-group-id}/message` | `group/ext-group-team1/message` | 发到外部群组 |
| 广播 | `broadcast/message` | `broadcast/message` | 广播给所有 |

### 4.3 订阅策略

转发器通过外部传入的 ID 列表订阅特定的 Topic，而不是使用通配符 `+`：

```go
// 外部传入的 ID 列表示例
externalAgentIDs := []string{"ext-agent-x", "ext-agent-y"}
externalGroupIDs := []string{"ext-group-team1", "ext-group-team2"}

// 订阅外部 Agent 的消息（发给本机的）
for _, agentID := range externalAgentIDs {
    topic := fmt.Sprintf("agent/%s/message", agentID)
    c.mqttClient.Subscribe(topic, 1, externalAgentHandler)
}

// 订阅外部群组的消息
for _, groupID := range externalGroupIDs {
    topic := fmt.Sprintf("group/%s/message", groupID)
    c.mqttClient.Subscribe(topic, 1, externalGroupHandler)
}

// 订阅广播消息（固定 Topic）
c.mqttClient.Subscribe("broadcast/message", 1, broadcastHandler)
```

### 4.4 外部 ID 传入接口

转发器提供接口供外部传入需要订阅的 Agent ID 和群组 ID：

```go
// Connector 接口扩展
type Connector interface {
    // ... 其他方法
    
    // SubscribeExternalAgents 订阅外部 Agent 消息
    // 参数:
    //   - agentIDs: 外部 Agent ID 列表
    SubscribeExternalAgents(agentIDs []string) error
    
    // SubscribeExternalGroups 订阅外部群组消息
    // 参数:
    //   - groupIDs: 外部群组 ID 列表
    SubscribeExternalGroups(groupIDs []string) error
    
    // UnsubscribeExternalAgents 取消订阅外部 Agent
    UnsubscribeExternalAgents(agentIDs []string) error
    
    // UnsubscribeExternalGroups 取消订阅外部群组
    UnsubscribeExternalGroups(groupIDs []string) error
}
```

**使用示例**：

```go
// 创建转发器
connector, _ := agentconnector.NewConnector(config)

// 启动时传入需要订阅的外部 ID
connector.SubscribeExternalAgents([]string{"ext-agent-x", "ext-agent-y"})
connector.SubscribeExternalGroups([]string{"ext-group-team1"})

// 动态添加订阅
connector.SubscribeExternalAgents([]string{"ext-agent-z"})

// 动态取消订阅
connector.UnsubscribeExternalAgents([]string{"ext-agent-x"})
```

---

## 5. 通信场景详细设计

### 5.1 场景 1：本机 Agent → 外部 Agent

**流程**：本机 Agent 发送消息给外部 Agent X

#### 步骤 1：本机 Agent 发送（WebSocket）

```json
{
  "type": "message.send",
  "session_id": "agent:ext-agent-x",
  "payload": {
    "content": "Hello External Agent X!"
  }
}
```

#### 步骤 2：转发器处理

```go
func (c *connector) handleWebSocketMessage(picoMsg PicoMessage) error {
    // 1. 解析 SessionID 获取 Target
    _, targetID := protocol.ParseSessionID(picoMsg.SessionID)
    // targetID = "ext-agent-x"
    
    // 2. 构建 MQTT Topic
    topic := fmt.Sprintf("agent/%s/message", targetID)
    // topic = "agent/ext-agent-x/message"
    
    // 3. 提取 content 作为 MQTT 消息（纯文本）
    content := picoMsg.Payload["content"].(string)
    // content = "Hello External Agent X!"
    
    // 4. 发布到 MQTT
    return c.mqttClient.Publish(topic, 1, false, content)
}
```

#### 步骤 3：MQTT 消息

```
Topic: agent/ext-agent-x/message
Payload: Hello External Agent X!  (纯文本，无 JSON 包装)
```

---

### 5.2 场景 2：外部 Agent → 本机 Agent

**流程**：外部 Agent X 发送消息给本机 Agent

#### 步骤 1：外部 Agent X 发送（MQTT）

```
Topic: agent/ext-agent-x/message
Payload: Hello Local Agent!  (纯文本)
```

#### 步骤 2：转发器接收并处理

```go
func (c *connector) handleMQTTMessage(topic string, data []byte) error {
    // 1. 从 Topic 生成 SessionID
    sessionID := protocol.TopicToSessionID(topic)
    // sessionID = "agent:ext-agent-x"
    
    // 2. MQTT 消息直接作为 content
    content := string(data)
    // content = "Hello Local Agent!"
    
    // 3. 构建 PicoMessage
    picoMsg := PicoMessage{
        Type:      "message.send",
        SessionID: sessionID,
        Payload: map[string]any{
            "content": content,
        },
    }
    
    // 4. 发送给本机 Agent（WebSocket）
    return c.wsClient.SendPicoMessage(picoMsg)
}
```

#### 步骤 3：本机 Agent 接收（WebSocket）

```json
{
  "type": "message.send",
  "session_id": "agent:ext-agent-x",
  "payload": {
    "content": "Hello Local Agent!"
  }
}
```

---

### 5.3 场景 3：本机 Agent → 外部群组

**流程**：本机 Agent 发送消息到外部群组 G

#### 步骤 1：本机 Agent 发送（WebSocket）

```json
{
  "type": "message.send",
  "session_id": "group:ext-group-team1",
  "payload": {
    "content": "Hello Team!"
  }
}
```

#### 步骤 2：转发器处理

```go
func (c *connector) handleWebSocketMessage(picoMsg PicoMessage) error {
    // 1. 解析 SessionID
    _, targetID := protocol.ParseSessionID(picoMsg.SessionID)
    // targetID = "ext-group-team1"
    
    // 2. 构建 MQTT Topic（群聊）
    topic := fmt.Sprintf("group/%s/message", targetID)
    // topic = "group/ext-group-team1/message"
    
    // 3. 提取 content
    content := picoMsg.Payload["content"].(string)
    
    // 4. 发布到 MQTT
    return c.mqttClient.Publish(topic, 1, false, content)
}
```

#### 步骤 3：MQTT 消息

```
Topic: group/ext-group-team1/message
Payload: Hello Team!  (纯文本)
```

---

### 5.4 场景 4：外部群组 → 本机 Agent

**流程**：外部群组中有消息，转发给本机订阅的 Agents

#### 步骤 1：转发器订阅群组消息

```go
c.mqttClient.Subscribe("group/+/message", 1, groupMessageHandler)
```

#### 步骤 2：收到群组消息

```go
func (c *connector) handleGroupMessage(topic string, data []byte) error {
    // 1. 从 Topic 获取群组 ID 和生成 SessionID
    sessionID := protocol.TopicToSessionID(topic)
    // sessionID = "group:ext-group-team1"
    
    // 2. MQTT 消息作为 content
    content := string(data)
    
    // 3. 获取本机订阅该群组的 Agents（实际实现中需要维护订阅关系）
    // 注意：这里假设转发器知道哪些本机 Agent 订阅了这个群组
    // 实际可能需要额外的订阅管理模块
    
    // 4. 构建 PicoMessage
    picoMsg := PicoMessage{
        Type:      "message.send",
        SessionID: sessionID,
        Payload: map[string]any{
            "content": content,
        },
    }
    
    // 5. 发送给本机 Agent
    return c.wsClient.SendPicoMessage(picoMsg)
}
```

---

## 6. 核心代码实现

### 6.1 WebSocket → MQTT 转发

```go
package agentconnector

import (
    "fmt"
    "github.com/jdhxyy/agent-connector/protocol"
)

// forwardWebSocketToMQTT 将 WebSocket 消息转发到 MQTT
func (c *connector) forwardWebSocketToMQTT(picoMsg protocol.PicoMessage) error {
    // 1. 解析 SessionID 获取 Target
    _, targetID := protocol.ParseSessionID(picoMsg.SessionID)
    if targetID == "" {
        return fmt.Errorf("invalid session_id: %s", picoMsg.SessionID)
    }
    
    // 2. 根据 Target 类型构建 Topic
    var topic string
    switch {
    case strings.HasPrefix(targetID, "ext-agent-"):
        topic = fmt.Sprintf("agent/%s/message", targetID)
    case strings.HasPrefix(targetID, "ext-group-"):
        topic = fmt.Sprintf("group/%s/message", targetID)
    default:
        return fmt.Errorf("unknown target type: %s", targetID)
    }
    
    // 3. 提取 content 作为 MQTT 消息（纯文本）
    content, ok := picoMsg.Payload["content"].(string)
    if !ok {
        return fmt.Errorf("invalid payload content")
    }
    
    // 4. 发布到 MQTT
    return c.mqttClient.Publish(topic, 1, false, content)
}
```

### 6.2 MQTT → WebSocket 转发

```go
// forwardMQTTToWebSocket 将 MQTT 消息转发到 WebSocket
func (c *connector) forwardMQTTToWebSocket(topic string, data []byte) error {
    // 1. 从 Topic 生成 SessionID
    sessionID := protocol.TopicToSessionID(topic)
    if sessionID == "" {
        return fmt.Errorf("invalid topic: %s", topic)
    }
    
    // 2. MQTT 消息直接作为 content
    content := string(data)
    
    // 3. 构建 PicoMessage
    picoMsg := protocol.PicoMessage{
        Type:      "message.send",
        SessionID: sessionID,
        Payload: map[string]any{
            "content": content,
        },
    }
    
    // 4. 发送给本机 Agent
    return c.wsClient.SendPicoMessage(picoMsg)
}
```

### 6.3 协议工具函数

```go
package protocol

import "strings"

// TopicToSessionID 从 MQTT Topic 生成 SessionID
func TopicToSessionID(topic string) string {
    parts := strings.Split(topic, "/")
    if len(parts) < 2 {
        return ""
    }
    
    switch parts[0] {
    case "agent":
        return "agent:" + parts[1]
    case "group":
        return "group:" + parts[1]
    case "broadcast":
        return "broadcast:all"
    }
    return ""
}

// ParseSessionID 解析 SessionID
func ParseSessionID(sessionID string) (targetType, targetID string) {
    parts := strings.SplitN(sessionID, ":", 2)
    if len(parts) == 2 {
        return parts[0], parts[1]
    }
    return "", ""
}
```

### 6.4 订阅管理接口实现

```go
// SubscribeExternalAgents 订阅外部 Agent 消息
func (c *connector) SubscribeExternalAgents(agentIDs []string) error {
    for _, agentID := range agentIDs {
        topic := fmt.Sprintf("agent/%s/message", agentID)
        if err := c.mqttClient.Subscribe(topic, 1, c.mqttMessageHandler); err != nil {
            return fmt.Errorf("subscribe to agent %s failed: %w", agentID, err)
        }
        // 记录已订阅的 Agent
        c.subscribedAgents[agentID] = true
    }
    return nil
}

// SubscribeExternalGroups 订阅外部群组消息
func (c *connector) SubscribeExternalGroups(groupIDs []string) error {
    for _, groupID := range groupIDs {
        topic := fmt.Sprintf("group/%s/message", groupID)
        if err := c.mqttClient.Subscribe(topic, 1, c.mqttMessageHandler); err != nil {
            return fmt.Errorf("subscribe to group %s failed: %w", groupID, err)
        }
        // 记录已订阅的群组
        c.subscribedGroups[groupID] = true
    }
    return nil
}

// UnsubscribeExternalAgents 取消订阅外部 Agent
func (c *connector) UnsubscribeExternalAgents(agentIDs []string) error {
    for _, agentID := range agentIDs {
        topic := fmt.Sprintf("agent/%s/message", agentID)
        if err := c.mqttClient.Unsubscribe(topic); err != nil {
            return fmt.Errorf("unsubscribe from agent %s failed: %w", agentID, err)
        }
        delete(c.subscribedAgents, agentID)
    }
    return nil
}

// UnsubscribeExternalGroups 取消订阅外部群组
func (c *connector) UnsubscribeExternalGroups(groupIDs []string) error {
    for _, groupID := range groupIDs {
        topic := fmt.Sprintf("group/%s/message", groupID)
        if err := c.mqttClient.Unsubscribe(topic); err != nil {
            return fmt.Errorf("unsubscribe from group %s failed: %w", groupID, err)
        }
        delete(c.subscribedGroups, groupID)
    }
    return nil
}

// mqttMessageHandler 统一的 MQTT 消息处理函数
func (c *connector) mqttMessageHandler(topic string, data []byte) {
    if err := c.forwardMQTTToWebSocket(topic, data); err != nil {
        log.Printf("Failed to forward MQTT message: %v", err)
    }
}
```

---

## 7. 总结

### 7.1 核心设计

1. **极简 MQTT 消息格式**
   - 纯文本 Payload，无 JSON 包装
   - 所有路由信息通过 Topic 传递
   - SessionID 由 Topic 自动生成

2. **清晰的 Topic 设计**
   - 外部 Agent: `agent/{external-agent-id}/message`
   - 外部群组: `group/{external-group-id}/message`
   - 广播: `broadcast/message`

3. **SessionID 自动生成**
   - `agent/{id}` → `agent:{id}`
   - `group/{id}` → `group:{id}`
   - `broadcast` → `broadcast:all`

### 7.2 通信矩阵

| 发送方 | 接收方 | 路径 | SessionID 生成方式 |
|--------|--------|------|-------------------|
| 本机 Agent | 外部 Agent | WebSocket → MQTT | 从 SessionID 解析 |
| 外部 Agent | 本机 Agent | MQTT → WebSocket | 从 Topic 生成 |
| 本机 Agent | 外部群组 | WebSocket → MQTT | 从 SessionID 解析 |
| 外部群组 | 本机 Agent | MQTT → WebSocket | 从 Topic 生成 |

### 7.3 优势

1. **极致简洁** - MQTT 消息无包装，直接传输文本
2. **无冗余** - Topic 携带所有路由信息
3. **自动映射** - SessionID 由 Topic 自动生成
4. **易于实现** - 代码逻辑清晰简单
5. **完全兼容** - 不修改 PicoMessage 协议

### 7.4 注意事项

1. **消息类型** - 目前只支持文本消息，如需支持其他类型，可通过 Topic 扩展（如 `agent/{id}/image`）
2. **消息 ID** - 如需消息去重，可在 PicoMessage 的 ID 字段中添加
3. **群聊订阅** - 需要额外维护本机 Agent 对群组的订阅关系

---

**文档结束**
