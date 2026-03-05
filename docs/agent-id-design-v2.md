# Agent 转发器 ID 设计方案 V2

## 文档信息

- **版本**: v2.0
- **日期**: 2026-03-04
- **作者**: AI Assistant
- **状态**: 修订版（适配严格协议限制）

---

## 1. 背景与约束条件

### 1.1 不可更改的协议定义

**PicoMessage 协议（严格固定）**：

```go
type PicoMessage struct {
    Type      string         `json:"type"`       // 消息类型
    ID        string         `json:"id,omitempty"` // 消息 ID（可选）
    SessionID string         `json:"session_id,omitempty"` // 会话 ID（可选）
    Timestamp int64          `json:"timestamp,omitempty"` // 时间戳（可选）
    Payload   map[string]any `json:"payload,omitempty"` // 消息负载（可选）
}
```

**严格约束**：
1. Payload 只能是 `{"content": "string"}` 格式
2. 不能添加自定义字段到 Payload
3. 不能修改 PicoMessage 结构
4. 必须通过现有字段传递所有信息

### 1.2 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│                    Agent 转发器 (Connector)                   │
├─────────────────────────┬───────────────────────────────────┤
│   WebSocket (Pico协议)   │          MQTT                     │
│   - 连接本地 Agent       │   - 连接外部 Agent                 │
│   - Payload: {content}   │   - Topic 携带目标信息              │
│   - SessionID 标识会话   │   - Payload 可扩展                  │
└─────────────────────────┴───────────────────────────────────┘
```

---

## 2. 核心设计思路

### 2.1 信息传递策略

由于 PicoMessage Payload 只能包含 `content` 字段，我们需要通过其他方式传递目标信息：

| 信息类型 | 传递方式 | 说明 |
|---------|---------|------|
| **目标 Agent ID** | MQTT Topic | `agent/chat/{target-id}/message` |
| **目标 Group ID** | MQTT Topic | `agent/group/{group-id}/message` |
| **消息类型** | MQTT Topic | Topic 最后一级：message/command/status |
| **来源 Agent ID** | WebSocket 连接标识 | 转发器记录连接与 Agent 的映射 |
| **会话标识** | SessionID 字段 | PicoMessage.SessionID |

### 2.2 关键洞察

**WebSocket 场景**：
- 转发器维护 `WebSocket连接 <-> AgentID` 的映射表
- 收到消息时，通过连接识别 Source
- 通过业务逻辑决定 Target（如从 URL 参数或上下文获取）

**MQTT 场景**：
- Topic 天然携带目标信息
- Payload 可以扩展，不受限制
- 需要 Topic → Target 的解析规则

---

## 3. ID 设计方案

### 3.1 ID 格式定义

| ID 类型 | 格式 | 示例 | 说明 |
|---------|------|------|------|
| **Agent ID** | `agent-{name}` | `agent-user1`, `agent-bot` | 用户/Agent 标识 |
| **Group ID** | `group-{name}` | `group-team1` | 群组标识 |
| **Session ID** | `sess-{agent-id}-{timestamp}` | `sess-agent-user1-1772632999` | 会话标识 |

### 3.2 SessionID 设计（关键）

SessionID 是 PicoMessage 中唯一可自由定义的可选字段，用于传递上下文信息：

```
SessionID 格式: {source-agent-id}:{target-type}:{target-id}

示例:
- 单聊: "agent-user1:agent:agent-user2"
- 群聊: "agent-user1:group:group-team1"
- 广播: "agent-user1:broadcast:all"
```

**解析规则**：
```go
func ParseSessionID(sessionID string) (source, targetType, target string) {
    parts := strings.Split(sessionID, ":")
    if len(parts) >= 3 {
        return parts[0], parts[1], parts[2]
    }
    return "", "", ""
}
```

---

## 4. 通信场景详细设计

### 4.1 单聊场景（Direct Message）

#### WebSocket → MQTT 转发

**步骤 1**: Agent A 发送消息（WebSocket）

```json
{
  "type": "message.send",
  "id": "msg-001",
  "session_id": "agent-user1:agent:agent-user2",
  "timestamp": 1772632999000,
  "payload": {
    "content": "Hello User2!"
  }
}
```

**步骤 2**: 转发器处理

```go
// 1. 从 WebSocket 连接获取 Source Agent ID
sourceID := c.getAgentIDByWebSocket(conn)  // "agent-user1"

// 2. 从 SessionID 解析 Target
_, targetType, targetID := ParseSessionID(picoMsg.SessionID)
// targetType = "agent", targetID = "agent-user2"

// 3. 构建 MQTT Topic
topic := fmt.Sprintf("agent/chat/%s/message", targetID)
// topic = "agent/chat/agent-user2/message"

// 4. 构建 MQTT Payload（可扩展）
mqttPayload := GenericMessage{
    ID:        picoMsg.ID,
    MsgType:   picoMsg.Type,
    Source:    sourceID,
    Target:    targetID,
    Payload:   []byte(`{"content":"Hello User2!"}`),
    Timestamp: time.UnixMilli(picoMsg.Timestamp),
    Metadata: map[string]string{
        "session_id": picoMsg.SessionID,
        "target_type": targetType,
    },
}

// 5. 发布到 MQTT
c.mqttClient.Publish(topic, 1, false, mqttPayload)
```

**步骤 3**: MQTT 消息格式

```json
{
  "ID": "msg-001",
  "MsgType": "message.send",
  "Source": "agent-user1",
  "Target": "agent-user2",
  "Payload": "eyJjb250ZW50IjoiSGVsbG8gVXNlcjIhIn0=",
  "Timestamp": "2026-03-04T10:30:00Z",
  "Metadata": {
    "session_id": "agent-user1:agent:agent-user2",
    "target_type": "agent"
  }
}
```

#### MQTT → WebSocket 转发

**步骤 1**: 转发器订阅 Topic

```go
// 订阅所有可能的目标 Topic
c.mqttClient.Subscribe("agent/chat/+/+", 1, handler)
c.mqttClient.Subscribe("agent/group/+/+", 1, handler)
```

**步骤 2**: 收到 MQTT 消息

```go
func (c *connector) handleMQTTMessage(topic string, payload []byte) {
    // 1. 解析 Topic 获取 Target
    // topic = "agent/chat/agent-user2/message"
    targetID := extractTargetFromTopic(topic)  // "agent-user2"
    
    // 2. 解析 MQTT Payload
    var msg GenericMessage
    json.Unmarshal(payload, &msg)
    
    // 3. 查找目标 Agent 的 WebSocket 连接
    wsConn := c.getWebSocketByAgentID(targetID)
    if wsConn == nil {
        log.Printf("Agent %s 不在线", targetID)
        return
    }
    
    // 4. 构建 PicoMessage
    picoMsg := PicoMessage{
        Type:      msg.MsgType,
        ID:        msg.ID,
        SessionID: msg.Metadata["session_id"],
        Timestamp: msg.Timestamp.UnixMilli(),
        Payload: map[string]any{
            "content": extractContent(msg.Payload),
        },
    }
    
    // 5. 发送给目标 Agent
    wsConn.SendPicoMessage(picoMsg)
}
```

**步骤 3**: Agent B 收到消息（WebSocket）

```json
{
  "type": "message.send",
  "id": "msg-001",
  "session_id": "agent-user1:agent:agent-user2",
  "timestamp": 1772632999000,
  "payload": {
    "content": "Hello User2!"
  }
}
```

### 4.2 群聊场景（Group Chat）

#### WebSocket → MQTT 转发

**步骤 1**: Agent A 发送群消息（WebSocket）

```json
{
  "type": "message.send",
  "id": "msg-002",
  "session_id": "agent-user1:group:group-team1",
  "timestamp": 1772632999000,
  "payload": {
    "content": "Hello Team!"
  }
}
```

**步骤 2**: 转发器处理

```go
// 1. 解析 SessionID
_, targetType, groupID := ParseSessionID(picoMsg.SessionID)
// targetType = "group", groupID = "group-team1"

// 2. 构建 MQTT Topic（群聊 Topic）
topic := fmt.Sprintf("agent/group/%s/message", groupID)
// topic = "agent/group/group-team1/message"

// 3. 发布到 MQTT
c.mqttClient.Publish(topic, 1, false, mqttPayload)
```

**步骤 3**: MQTT 消息格式

```json
{
  "ID": "msg-002",
  "MsgType": "message.send",
  "Source": "agent-user1",
  "Target": "group-team1",
  "Payload": "eyJjb250ZW50IjoiSGVsbG8gVGVhbSEifQ==",
  "Timestamp": "2026-03-04T10:30:00Z",
  "Metadata": {
    "session_id": "agent-user1:group:group-team1",
    "target_type": "group"
  }
}
```

#### MQTT → WebSocket 转发（群聊特殊处理）

**步骤 1**: 转发器订阅群聊 Topic

```go
c.mqttClient.Subscribe("agent/group/+/message", 1, groupMessageHandler)
```

**步骤 2**: 收到群聊消息

```go
func (c *connector) handleGroupMessage(topic string, payload []byte) {
    // 1. 解析 Group ID
    groupID := extractGroupFromTopic(topic)  // "group-team1"
    
    // 2. 解析消息
    var msg GenericMessage
    json.Unmarshal(payload, &msg)
    
    // 3. 获取群成员列表
    members := c.groupManager.GetGroupMembers(groupID)
    // members = ["agent-user1", "agent-user2", "agent-user3"]
    
    // 4. 转发给所有在线成员（除发送者）
    for _, memberID := range members {
        if memberID == msg.Source {
            continue  // 不转发给发送者
        }
        
        wsConn := c.getWebSocketByAgentID(memberID)
        if wsConn != nil {
            picoMsg := PicoMessage{
                Type:      msg.MsgType,
                ID:        msg.ID,
                SessionID: msg.Metadata["session_id"],
                Timestamp: msg.Timestamp.UnixMilli(),
                Payload: map[string]any{
                    "content": extractContent(msg.Payload),
                },
            }
            wsConn.SendPicoMessage(picoMsg)
        }
    }
}
```

### 4.3 广播场景（Broadcast）

#### WebSocket → MQTT 转发

**步骤 1**: Agent A 发送广播（WebSocket）

```json
{
  "type": "message.send",
  "id": "msg-003",
  "session_id": "agent-user1:broadcast:all",
  "timestamp": 1772632999000,
  "payload": {
    "content": "Broadcast to all!"
  }
}
```

**步骤 2**: 转发器处理

```go
// 1. 识别为广播消息
_, targetType, _ := ParseSessionID(picoMsg.SessionID)
// targetType = "broadcast"

// 2. 构建广播 Topic
topic := "agent/chat/broadcast/message"

// 3. 发布到 MQTT
c.mqttClient.Publish(topic, 1, false, mqttPayload)
```

---

## 5. MQTT Topic 设计（最终版）

### 5.1 Topic 层级结构

```
agent/                          # 根主题
├── chat/                       # 单聊消息
│   ├── {agent-id}/             # 目标 Agent ID
│   │   ├── message             # 文本消息
│   │   ├── command             # 命令消息
│   │   └── status              # 状态消息
│   └── broadcast/              # 广播消息
│       └── message             # 广播文本
├── group/                      # 群聊消息
│   └── {group-id}/             # 群组 ID
│       └── message             # 群聊消息
└── system/                     # 系统消息
    └── broadcast               # 系统广播
```

### 5.2 Topic 解析规则

```go
// TopicParser Topic 解析器
type TopicParser struct{}

// Parse 解析 Topic 返回目标信息
func (p *TopicParser) Parse(topic string) (targetType, targetID, msgType string) {
    parts := strings.Split(topic, "/")
    
    if len(parts) < 3 {
        return "", "", ""
    }
    
    switch parts[1] {
    case "chat":
        if parts[2] == "broadcast" {
            return "broadcast", "all", parts[3]
        }
        return "agent", parts[2], parts[3]
    case "group":
        return "group", parts[2], parts[3]
    case "system":
        return "system", "all", parts[2]
    }
    
    return "", "", ""
}
```

### 5.3 Topic 对照表

| 场景 | Topic 格式 | 示例 | 说明 |
|------|-----------|------|------|
| 单聊文本 | `agent/chat/{agent-id}/message` | `agent/chat/agent-user2/message` | 发送给指定 Agent |
| 单聊命令 | `agent/chat/{agent-id}/command` | `agent/chat/agent-user2/command` | 发送命令 |
| 单聊状态 | `agent/chat/{agent-id}/status` | `agent/chat/agent-user2/status` | 发送状态 |
| 广播 | `agent/chat/broadcast/message` | `agent/chat/broadcast/message` | 广播给所有 Agent |
| 群聊 | `agent/group/{group-id}/message` | `agent/group/group-team1/message` | 发送到群组 |
| 系统广播 | `agent/system/broadcast` | `agent/system/broadcast` | 系统级广播 |

---

## 6. SessionID 编码规范

### 6.1 格式定义

```
SessionID = {source-agent-id}:{target-type}:{target-id}

其中:
- source-agent-id: 发送者 Agent ID
- target-type: 目标类型 (agent/group/broadcast)
- target-id: 目标 ID
```

### 6.2 编码/解码函数

```go
package protocol

import "strings"

// EncodeSessionID 编码 SessionID
func EncodeSessionID(source, targetType, target string) string {
    return source + ":" + targetType + ":" + target
}

// DecodeSessionID 解码 SessionID
func DecodeSessionID(sessionID string) (source, targetType, target string) {
    parts := strings.SplitN(sessionID, ":", 3)
    if len(parts) >= 3 {
        return parts[0], parts[1], parts[2]
    }
    return "", "", ""
}

// SessionID 构建辅助函数
func NewDirectSession(source, targetAgent string) string {
    return EncodeSessionID(source, "agent", targetAgent)
}

func NewGroupSession(source, group string) string {
    return EncodeSessionID(source, "group", group)
}

func NewBroadcastSession(source string) string {
    return EncodeSessionID(source, "broadcast", "all")
}
```

### 6.3 使用示例

```go
// 单聊
sessionID := NewDirectSession("agent-user1", "agent-user2")
// 结果: "agent-user1:agent:agent-user2"

// 群聊
sessionID := NewGroupSession("agent-user1", "group-team1")
// 结果: "agent-user1:group:group-team1"

// 广播
sessionID := NewBroadcastSession("agent-user1")
// 结果: "agent-user1:broadcast:all"
```

---

## 7. 核心代码实现

### 7.1 WebSocket → MQTT 转发

```go
// forwardWebSocketToMQTT 将 WebSocket 消息转发到 MQTT
func (c *connector) forwardWebSocketToMQTT(picoMsg protocol.PicoMessage, conn *websocket.Conn) error {
    // 1. 获取 Source Agent ID
    sourceID := c.getAgentIDByConnection(conn)
    if sourceID == "" {
        return fmt.Errorf("unknown connection")
    }
    
    // 2. 从 SessionID 解析 Target
    _, targetType, targetID := protocol.DecodeSessionID(picoMsg.SessionID)
    if targetID == "" {
        return fmt.Errorf("invalid session_id")
    }
    
    // 3. 构建 MQTT Topic
    var topic string
    switch targetType {
    case "agent":
        topic = fmt.Sprintf("agent/chat/%s/%s", targetID, picoMsg.Type)
    case "group":
        topic = fmt.Sprintf("agent/group/%s/%s", targetID, picoMsg.Type)
    case "broadcast":
        topic = fmt.Sprintf("agent/chat/broadcast/%s", picoMsg.Type)
    default:
        return fmt.Errorf("unknown target type: %s", targetType)
    }
    
    // 4. 构建 MQTT Payload
    payload, _ := json.Marshal(picoMsg.Payload)
    mqttMsg := &protocol.GenericMessage{
        ID:        picoMsg.ID,
        MsgType:   picoMsg.Type,
        Source:    sourceID,
        Target:    targetID,
        Payload:   payload,
        Timestamp: time.UnixMilli(picoMsg.Timestamp),
        Metadata: map[string]string{
            "session_id": picoMsg.SessionID,
            "target_type": targetType,
        },
    }
    
    // 5. 发布到 MQTT
    mqttPayload, _ := json.Marshal(mqttMsg)
    return c.mqttClient.Publish(topic, 1, false, mqttPayload)
}
```

### 7.2 MQTT → WebSocket 转发

```go
// forwardMQTTToWebSocket 将 MQTT 消息转发到 WebSocket
func (c *connector) forwardMQTTToWebSocket(topic string, data []byte) error {
    // 1. 解析 Topic
    parser := &TopicParser{}
    targetType, targetID, _ := parser.Parse(topic)
    
    // 2. 解析 MQTT 消息
    var mqttMsg protocol.GenericMessage
    if err := json.Unmarshal(data, &mqttMsg); err != nil {
        return err
    }
    
    // 3. 根据目标类型处理
    switch targetType {
    case "agent":
        // 单聊：转发给指定 Agent
        return c.sendToAgent(targetID, mqttMsg)
    case "group":
        // 群聊：转发给群成员
        return c.sendToGroup(targetID, mqttMsg)
    case "broadcast":
        // 广播：转发给所有在线 Agent
        return c.sendToAll(mqttMsg)
    }
    
    return nil
}

// sendToAgent 发送给指定 Agent
func (c *connector) sendToAgent(agentID string, msg protocol.GenericMessage) error {
    conn := c.getConnectionByAgentID(agentID)
    if conn == nil {
        return fmt.Errorf("agent %s not online", agentID)
    }
    
    picoMsg := protocol.PicoMessage{
        Type:      msg.MsgType,
        ID:        msg.ID,
        SessionID: msg.Metadata["session_id"],
        Timestamp: msg.Timestamp.UnixMilli(),
        Payload:   map[string]any{"content": string(msg.Payload)},
    }
    
    return conn.SendPicoMessage(picoMsg)
}

// sendToGroup 发送给群成员
func (c *connector) sendToGroup(groupID string, msg protocol.GenericMessage) error {
    members := c.groupManager.GetMembers(groupID)
    for _, memberID := range members {
        if memberID == msg.Source {
            continue  // 跳过发送者
        }
        c.sendToAgent(memberID, msg)
    }
    return nil
}

// sendToAll 广播给所有 Agent
func (c *connector) sendToAll(msg protocol.GenericMessage) error {
    for agentID, conn := range c.connections {
        if agentID == msg.Source {
            continue  // 跳过发送者
        }
        picoMsg := protocol.PicoMessage{
            Type:      msg.MsgType,
            ID:        msg.ID,
            SessionID: msg.Metadata["session_id"],
            Timestamp: msg.Timestamp.UnixMilli(),
            Payload:   map[string]any{"content": string(msg.Payload)},
        }
        conn.SendPicoMessage(picoMsg)
    }
    return nil
}
```

---

## 8. 连接管理

### 8.1 WebSocket 连接映射

```go
type ConnectionManager struct {
    // WebSocket连接 -> AgentID
    connToAgent map[*websocket.Conn]string
    // AgentID -> WebSocket连接
    agentToConn map[string]*websocket.Conn
    mu          sync.RWMutex
}

func (cm *ConnectionManager) Register(conn *websocket.Conn, agentID string) {
    cm.mu.Lock()
    defer cm.mu.Unlock()
    cm.connToAgent[conn] = agentID
    cm.agentToConn[agentID] = conn
}

func (cm *ConnectionManager) GetAgentID(conn *websocket.Conn) string {
    cm.mu.RLock()
    defer cm.mu.RUnlock()
    return cm.connToAgent[conn]
}

func (cm *ConnectionManager) GetConnection(agentID string) *websocket.Conn {
    cm.mu.RLock()
    defer cm.mu.RUnlock()
    return cm.agentToConn[agentID]
}
```

### 8.2 Agent 上线/下线处理

```go
// OnAgentConnect Agent 连接时
func (c *connector) OnAgentConnect(conn *websocket.Conn, agentID string) {
    c.connManager.Register(conn, agentID)
    
    // 订阅该 Agent 的 MQTT Topic
    topic := fmt.Sprintf("agent/chat/%s/+", agentID)
    c.mqttClient.Subscribe(topic, 1, c.mqttHandler)
    
    log.Printf("Agent %s connected", agentID)
}

// OnAgentDisconnect Agent 断开时
func (c *connector) OnAgentDisconnect(conn *websocket.Conn) {
    agentID := c.connManager.GetAgentID(conn)
    c.connManager.Unregister(conn)
    
    log.Printf("Agent %s disconnected", agentID)
}
```

---

## 9. 完整数据流

### 9.1 单聊数据流

```
Agent A (WebSocket)                    转发器                    MQTT Broker                    Agent B (WebSocket)
     │                                    │                            │                              │
     │ 1. 发送消息                        │                            │                              │
     │ {                                │                            │                              │
     │   "type": "message.send",        │                            │                              │
     │   "session_id": "A:agent:B",     │                            │                              │
     │   "payload": {"content":"Hi"}    │                            │                              │
     │ }                                │                            │                              │
     │─────────────────────────────────>│                            │                              │
     │                                    │ 2. 解析 SessionID          │                              │
     │                                    │    Source=A, Target=B      │                              │
     │                                    │                            │                              │
     │                                    │ 3. 构建 MQTT 消息          │                              │
     │                                    │    Topic: agent/chat/B/msg │                              │
     │                                    │───────────────────────────>│                              │
     │                                    │                            │                              │
     │                                    │                            │ 4. 转发给订阅者              │
     │                                    │                            │─────────────────────────────>│
     │                                    │                            │                              │
     │                                    │                            │                              │ 5. 接收并解析
     │                                    │                            │                              │    Source=A
     │                                    │                            │                              │    显示消息
```

### 9.2 群聊数据流

```
Agent A (WebSocket)                    转发器                    MQTT Broker              Agent B/C/D (WebSocket)
     │                                    │                            │                        │
     │ 1. 发送群消息                      │                            │                        │
     │ {                                │                            │                        │
     │   "type": "message.send",        │                            │                        │
     │   "session_id": "A:group:G",     │                            │                        │
     │   "payload": {"content":"Hi"}    │                            │                        │
     │ }                                │                            │                        │
     │─────────────────────────────────>│                            │                        │
     │                                    │ 2. 解析为群聊              │                        │
     │                                    │    Topic: agent/group/G/msg│                        │
     │                                    │───────────────────────────>│                        │
     │                                    │                            │                        │
     │                                    │                            │ 3. 转发                 │
     │                                    │<───────────────────────────│                        │
     │                                    │                            │                        │
     │                                    │ 4. 查询群成员 [B,C,D]      │                        │
     │                                    │ 5. 转发给在线成员          │                        │
     │                                    │────────────────────────────────────────────────────>│
     │                                    │                            │                        │
     │                                    │                            │                        │ 6. B/C/D 接收
```

---

## 10. 总结

### 10.1 核心设计要点

1. **SessionID 承载路由信息**
   - 格式: `{source}:{target-type}:{target-id}`
   - 在 PicoMessage 的 SessionID 字段传递
   - 不修改 Payload 结构

2. **MQTT Topic 承载目标信息**
   - 单聊: `agent/chat/{agent-id}/message`
   - 群聊: `agent/group/{group-id}/message`
   - 广播: `agent/chat/broadcast/message`

3. **连接管理维护映射关系**
   - WebSocket 连接 ↔ Agent ID
   - 用于识别消息来源和转发目标

### 10.2 优势

1. **完全兼容现有协议**
   - 不修改 PicoMessage 结构
   - Payload 保持 `{"content": "..."}` 格式

2. **清晰的职责分离**
   - SessionID: 端到端会话标识
   - Topic: MQTT 路由信息
   - Payload: 消息内容

3. **可扩展性**
   - 支持单聊、群聊、广播
   - 易于添加新的消息类型
   - 群组成员管理独立

### 10.3 注意事项

1. SessionID 长度限制（建议 < 256 字符）
2. Agent ID 和 Group ID 不能包含 `:` 字符
3. 需要维护 WebSocket 连接状态
4. 群聊需要额外的成员管理模块

---

**文档结束**
