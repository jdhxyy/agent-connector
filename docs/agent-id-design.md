# Agent 转发器 ID 设计方案

## 文档信息

- **版本**: v1.0
- **日期**: 2026-03-04
- **作者**: AI Assistant
- **状态**: 设计讨论稿

---

## 1. 背景与问题概述

### 1.1 系统架构

Agent 转发器作为中间件，连接两种通信协议：

```
┌─────────────────────────────────────────────────────────────┐
│                    Agent 转发器 (Connector)                   │
├─────────────────────────┬───────────────────────────────────┤
│   WebSocket (Pico协议)   │          MQTT                     │
│   - 连接本地 Agent       │   - 连接外部 Agent                 │
│   - 实时双向通信         │   - 发布/订阅模式                  │
│   - 点对点通信          │   - 支持广播                       │
└─────────────────────────┴───────────────────────────────────┘
```

### 1.2 核心问题

**PicoMessage 协议限制**（不可更改）：

```go
type PicoMessage struct {
    Type      string         `json:"type"`       // 消息类型
    ID        string         `json:"id,omitempty"` // 消息 ID（可选）
    SessionID string         `json:"session_id,omitempty"` // 会话 ID（可选）
    Timestamp int64          `json:"timestamp,omitempty"` // 时间戳（可选）
    Payload   map[string]any `json:"payload,omitempty"` // 消息负载（可选）
}
```

**问题点**：
1. 没有显式的 `Source` 和 `Target` 字段
2. 需要通过 `Payload` 传递目标信息
3. 需要区分单聊和群聊场景

---

## 2. ID 设计方案

### 2.1 ID 类型定义

| ID 类型 | 格式 | 示例 | 说明 |
|---------|------|------|------|
| **Agent ID** | `agent-{uuid}` 或 `agent-{name}` | `agent-abc123`, `agent-user1` | 用户/Agent 唯一标识 |
| **Group ID** | `group-{uuid}` 或 `group-{name}` | `group-team1`, `group-project-a` | 群组唯一标识 |
| **Session ID** | `session-{timestamp}-{random}` | `session-1772632999-abc123` | 会话标识（已存在）|
| **Message ID** | `msg-{timestamp}-{random}` | `msg-1772632999-xyz789` | 消息唯一标识 |

### 2.2 PicoMessage Payload 设计

为了兼容现有协议，在 `Payload` 中定义标准字段：

```go
// 单聊消息 Payload 结构
type DirectMessagePayload struct {
    Content    string `json:"content"`              // 消息内容
    TargetID   string `json:"target_id"`            // 目标 Agent ID
    TargetType string `json:"target_type"`          // 目标类型: "agent"
    ReplyTo    string `json:"reply_to,omitempty"`   // 回复的消息 ID（可选）
}

// 群聊消息 Payload 结构
type GroupMessagePayload struct {
    Content    string   `json:"content"`              // 消息内容
    TargetID   string   `json:"target_id"`            // 目标 Group ID
    TargetType string   `json:"target_type"`          // 目标类型: "group"
    Mentions   []string `json:"mentions,omitempty"`   // @提及的用户列表（可选）
    ReplyTo    string   `json:"reply_to,omitempty"`   // 回复的消息 ID（可选）
}

// 通用 Payload 结构（用于转发器内部）
type MessagePayload struct {
    Content    string   `json:"content"`              // 消息内容
    TargetID   string   `json:"target_id"`            // 目标 ID
    TargetType string   `json:"target_type"`          // 目标类型: "agent" 或 "group"
    SourceID   string   `json:"source_id"`            // 来源 Agent ID
    Mentions   []string `json:"mentions,omitempty"`   // @提及列表（群聊用）
    ReplyTo    string   `json:"reply_to,omitempty"`   // 回复引用
}
```

---

## 3. 通信场景分析

### 3.1 单聊场景 (Direct Message)

**场景描述**：Agent A 发送消息给 Agent B

#### WebSocket → MQTT 转发

```
1. Agent A (WebSocket) 发送消息:
   {
     "type": "message.send",
     "id": "msg-001",
     "session_id": "session-abc",
     "payload": {
       "content": "Hello B",
       "target_id": "agent-b",
       "target_type": "agent"
     }
   }

2. 转发器处理:
   - 从 WebSocket 连接获取 Source ID (agent-a)
   - 从 Payload 获取 Target ID (agent-b)
   - 转换为 MQTT 消息，发布到 Topic: agent/chat/agent-b/message

3. MQTT 消息格式:
   {
     "ID": "msg-001",
     "MsgType": "message.send",
     "Source": "agent-a",
     "Target": "agent-b",
     "Payload": "{\"content\":\"Hello B\"}",
     "Metadata": {
       "target_type": "agent",
       "session_id": "session-abc"
     }
   }
```

#### MQTT → WebSocket 转发

```
1. 转发器订阅 Topic: agent/chat/agent-b/#

2. 收到 MQTT 消息:
   Topic: agent/chat/agent-b/message
   Payload: {如上所示}

3. 转发器处理:
   - 解析 Topic 获取 Target (agent-b)
   - 解析 Payload 获取 Source (agent-a)
   - 转换为 PicoMessage，发送给 Agent B (WebSocket)

4. WebSocket 消息格式:
   {
     "type": "message.send",
     "id": "msg-001",
     "session_id": "session-abc",
     "payload": {
       "content": "Hello B",
       "source_id": "agent-a",
       "target_id": "agent-b",
       "target_type": "agent"
     }
   }
```

### 3.2 群聊场景 (Group Chat)

**场景描述**：Agent A 在 Group X 中发送消息

#### WebSocket → MQTT 转发

```
1. Agent A (WebSocket) 发送消息:
   {
     "type": "message.send",
     "id": "msg-002",
     "session_id": "session-xyz",
     "payload": {
       "content": "Hello everyone",
       "target_id": "group-x",
       "target_type": "group",
       "mentions": ["agent-b", "agent-c"]
     }
   }

2. 转发器处理:
   - 识别 target_type 为 "group"
   - 转换为 MQTT 消息，发布到 Topic: agent/group/group-x/message

3. MQTT 消息格式:
   {
     "ID": "msg-002",
     "MsgType": "message.send",
     "Source": "agent-a",
     "Target": "group-x",
     "Payload": "{\"content\":\"Hello everyone\"}",
     "Metadata": {
       "target_type": "group",
       "session_id": "session-xyz",
       "mentions": "agent-b,agent-c"
     }
   }
```

#### MQTT → WebSocket 转发

```
1. 转发器订阅 Topic: agent/group/group-x/#

2. 收到 MQTT 消息:
   Topic: agent/group/group-x/message
   Payload: {如上所示}

3. 转发器处理:
   - 识别为群聊消息
   - 查询 Group X 的成员列表
   - 转发给所有在线成员（除发送者外）

4. WebSocket 消息格式（发送给 Agent B）:
   {
     "type": "message.send",
     "id": "msg-002",
     "session_id": "session-xyz",
     "payload": {
       "content": "Hello everyone",
       "source_id": "agent-a",
       "target_id": "group-x",
       "target_type": "group",
       "is_mentioned": true  // 因为 Agent B 在 mentions 列表中
     }
   }
```

---

## 4. MQTT Topic 设计

### 4.1 Topic 层级结构

```
agent/                    # 根主题
├── chat/                 # 单聊消息
│   ├── {agent-id}/       # 目标 Agent ID
│   │   ├── message       # 文本消息
│   │   ├── command       # 命令消息
│   │   └── status        # 状态消息
│   └── broadcast/        # 广播消息
│       └── message       # 广播文本
├── group/                # 群聊消息
│   └── {group-id}/       # 群组 ID
│       └── message       # 群聊消息
└── system/               # 系统消息
    └── broadcast         # 系统广播
```

### 4.2 Topic 命名规范

| 场景 | Topic 格式 | 示例 |
|------|-----------|------|
| 单聊文本 | `agent/chat/{target-agent-id}/message` | `agent/chat/agent-b/message` |
| 单聊命令 | `agent/chat/{target-agent-id}/command` | `agent/chat/agent-b/command` |
| 单聊状态 | `agent/chat/{target-agent-id}/status` | `agent/chat/agent-b/status` |
| 广播消息 | `agent/chat/broadcast/message` | `agent/chat/broadcast/message` |
| 群聊消息 | `agent/group/{group-id}/message` | `agent/group/group-x/message` |
| 系统广播 | `agent/system/broadcast` | `agent/system/broadcast` |

---

## 5. 消息路由逻辑

### 5.1 路由决策流程

```
收到消息 (PicoMessage 或 MQTT Message)
    │
    ▼
提取 TargetID 和 TargetType
    │
    ├── TargetType = "agent" ──► 单聊路由
    │                              │
    │                              ▼
    │                         构建 Topic: agent/chat/{target-id}/message
    │                              │
    │                              ▼
    │                         发布到 MQTT
    │
    └── TargetType = "group" ──► 群聊路由
                                   │
                                   ▼
                              构建 Topic: agent/group/{group-id}/message
                                   │
                                   ▼
                              发布到 MQTT
                                   │
                                   ▼
                              群成员管理模块
                                   │
                                   ▼
                              转发给所有在线成员
```

### 5.2 代码实现示例

```go
// RouteMessage 消息路由函数
func (c *connector) RouteMessage(msg protocol.Message) error {
    targetID := msg.GetTarget()
    targetType := msg.GetMetadata()["target_type"]
    
    switch targetType {
    case "agent":
        // 单聊路由
        return c.routeDirectMessage(msg, targetID)
    case "group":
        // 群聊路由
        return c.routeGroupMessage(msg, targetID)
    default:
        // 默认按单聊处理
        return c.routeDirectMessage(msg, targetID)
    }
}

// routeDirectMessage 单聊路由
func (c *connector) routeDirectMessage(msg protocol.Message, targetID string) error {
    topic := fmt.Sprintf("agent/chat/%s/%s", targetID, msg.GetType())
    return c.SendToExternalAgent(targetID, msg)
}

// routeGroupMessage 群聊路由
func (c *connector) routeGroupMessage(msg protocol.Message, groupID string) error {
    // 1. 发布到群聊 Topic
    topic := fmt.Sprintf("agent/group/%s/message", groupID)
    if err := c.mqttClient.Publish(topic, 1, false, msg.GetPayload()); err != nil {
        return err
    }
    
    // 2. 获取群成员并转发
    members := c.getGroupMembers(groupID)
    for _, memberID := range members {
        if memberID != msg.GetSource() { // 不转发给发送者
            if err := c.SendToLocalAgent(memberID, msg); err != nil {
                log.Printf("Failed to forward to %s: %v", memberID, err)
            }
        }
    }
    return nil
}
```

---

## 6. 群组成员管理

### 6.1 数据结构

```go
// Group 群组信息
type Group struct {
    ID          string    // 群组 ID: group-{name}
    Name        string    // 群组名称
    CreatorID   string    // 创建者 Agent ID
    Members     []string  // 成员 Agent ID 列表
    CreatedAt   time.Time // 创建时间
    UpdatedAt   time.Time // 更新时间
}

// GroupManager 群组管理器
type GroupManager struct {
    groups map[string]*Group        // group-id -> Group
    memberGroups map[string][]string // agent-id -> []group-id
    mu     sync.RWMutex
}
```

### 6.2 群组操作

```go
// CreateGroup 创建群组
func (gm *GroupManager) CreateGroup(name, creatorID string, memberIDs []string) (*Group, error)

// JoinGroup 加入群组
func (gm *GroupManager) JoinGroup(groupID, agentID string) error

// LeaveGroup 离开群组
func (gm *GroupManager) LeaveGroup(groupID, agentID string) error

// GetGroupMembers 获取群成员
func (gm *GroupManager) GetGroupMembers(groupID string) []string

// GetAgentGroups 获取 Agent 加入的所有群组
func (gm *GroupManager) GetAgentGroups(agentID string) []string
```

---

## 7. 数据流图

### 7.1 单聊完整流程

```
┌─────────┐     WebSocket      ┌─────────────┐     MQTT Publish     ┌─────────┐
│ Agent A │ ─────────────────► │  转发器      │ ───────────────────► │  MQTT   │
│ (发送者)│  PicoMessage       │             │  Topic:              │  Broker │
└─────────┘  target: agent-b   │ 提取 target  │  agent/chat/agent-b/ │         │
                               │ 构建 MQTT    │  message             │         │
                               │ 消息         │                      │         │
                               └─────────────┘                      └────┬────┘
                                                                         │
                                                                         │ Subscribe
                                                                         │ agent/chat/agent-b/#
                                                                         ▼
                               ┌─────────────┐     WebSocket      ┌─────────┐
                               │  转发器      │ ◄───────────────── │  MQTT   │
                               │             │  接收 MQTT 消息    │  Broker │
                               │ 解析 Source │                    │         │
                               │ 转发给 B    │                    │         │
                               └──────┬──────┘                    └─────────┘
                                      │
                                      │ WebSocket
                                      ▼
                               ┌─────────┐
                               │ Agent B │
                               │ (接收者)│
                               └─────────┘
```

### 7.2 群聊完整流程

```
┌─────────┐     WebSocket      ┌─────────────┐     MQTT Publish     ┌─────────┐
│ Agent A │ ─────────────────► │  转发器      │ ───────────────────► │  MQTT   │
│ (发送者)│  PicoMessage       │             │  Topic:              │  Broker │
│         │  target: group-x   │ 识别群聊     │  agent/group/group-x/│         │
│         │  target_type: group│ 查询成员     │  message             │         │
└─────────┘                    │ 批量转发    │                      └────┬────┘
                               └─────────────┘                         │
                                                                        │ Subscribe
                                                                        │ agent/group/group-x/#
                                                                        ▼
                               ┌─────────────┐     WebSocket      ┌─────────┐
                               │  转发器      │ ◄───────────────── │  MQTT   │
                               │             │  接收 MQTT 消息    │  Broker │
                               │ 解析 Group  │                    │         │
                               │ 转发给成员  │                    │         │
                               └──────┬──────┘                    └─────────┘
                                      │
                    ┌─────────────────┼─────────────────┐
                    │                 │                 │
                    ▼                 ▼                 ▼
               ┌─────────┐      ┌─────────┐      ┌─────────┐
               │ Agent B │      │ Agent C │      │ Agent D │
               │(群成员) │      │(群成员) │      │(群成员) │
               └─────────┘      └─────────┘      └─────────┘
```

---

## 8. 边界情况处理

### 8.1 目标 Agent 离线

**场景**：Agent A 发送消息给离线的 Agent B

**处理方案**：
1. MQTT 消息正常发布到 Topic
2. Agent B 上线后，通过订阅历史消息获取（如果启用 retained 消息）
3. 或者通过离线消息队列存储，等 Agent B 上线后推送

### 8.2 群成员变动

**场景**：Agent 发送群消息时，有成员加入或离开

**处理方案**：
1. 发送时刻的成员列表决定接收者
2. 新加入的成员无法收到历史消息（除非启用消息持久化）
3. 离开的成员不再收到新消息

### 8.3 消息重复

**场景**：网络抖动导致消息重复发送

**处理方案**：
1. 使用 Message ID 去重
2. 客户端维护已接收消息 ID 列表
3. 重复消息丢弃

---

## 9. 安全考虑

### 9.1 身份验证

- WebSocket 连接使用 Token 认证
- MQTT 连接使用用户名/密码认证
- Agent ID 与认证信息绑定，防止伪造

### 9.2 权限控制

- 单聊：只能发送给已授权的 Agent
- 群聊：只能发送给已加入的群组
- 系统消息：只有管理员可以发送

### 9.3 消息加密

- WebSocket 使用 WSS（WebSocket Secure）
- MQTT 使用 TLS 加密
- 敏感消息内容可端到端加密

---

## 10. 总结

### 10.1 设计要点

1. **兼容现有协议**：通过 Payload 传递 Target 信息，不修改 PicoMessage 结构
2. **清晰的 ID 体系**：区分 Agent ID 和 Group ID，使用命名前缀区分类型
3. **灵活的路由**：支持单聊、群聊、广播多种场景
4. **可扩展性**：预留 Mentions、ReplyTo 等扩展字段

### 10.2 后续优化方向

1. **消息持久化**：支持离线消息和历史记录
2. **已读回执**：添加消息已读状态追踪
3. **消息撤回**：支持撤回已发送消息
4. **文件传输**：支持图片、文件等媒体消息
5. **消息搜索**：支持历史消息搜索

---

## 附录

### A. 消息类型对照表

| PicoMessage Type | MQTT MsgType | 说明 |
|-----------------|--------------|------|
| `message.send` | `message.send` | 发送文本消息 |
| `media.send` | `media.send` | 发送媒体消息 |
| `typing.start` | `typing.start` | 开始输入 |
| `typing.stop` | `typing.stop` | 结束输入 |
| `ping` | `ping` | 心跳请求 |
| `pong` | `pong` | 心跳响应 |

### B. 标准 Payload 字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `content` | string | 是 | 消息内容 |
| `target_id` | string | 是 | 目标 ID |
| `target_type` | string | 是 | 目标类型: "agent"/"group" |
| `source_id` | string | 否 | 来源 ID（服务器填充）|
| `mentions` | []string | 否 | @提及列表 |
| `reply_to` | string | 否 | 回复的消息 ID |
| `is_mentioned` | bool | 否 | 是否被@（服务器填充）|

---

**文档结束**
