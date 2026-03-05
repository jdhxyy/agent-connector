package router

import (
	"errors"
	"testing"

	"github.com/jdhxyy/agent-connector/protocol"
)

func TestNewRouter(t *testing.T) {
	router := NewRouter()
	if router == nil {
		t.Fatal("NewRouter() returned nil")
	}

	rules := router.ListRules()
	if len(rules) != 0 {
		t.Errorf("Initial rules count = %d, want 0", len(rules))
	}
}

func TestRouter_RegisterRule(t *testing.T) {
	router := NewRouter()

	rule := RouteRule{
		Source:      "websocket",
		SourceAgent: "agent-1",
		TargetAgent: "agent-2",
		MessageType: "text",
	}

	err := router.RegisterRule(rule)
	if err != nil {
		t.Errorf("RegisterRule() error = %v", err)
	}

	rules := router.ListRules()
	if len(rules) != 1 {
		t.Errorf("Rules count = %d, want 1", len(rules))
	}

	if rules[0].Source != "websocket" {
		t.Errorf("Rule.Source = %v, want websocket", rules[0].Source)
	}
}

func TestRouter_RegisterMultipleRules(t *testing.T) {
	router := NewRouter()

	rules := []RouteRule{
		{
			Source:      "websocket",
			SourceAgent: "agent-1",
			TargetAgent: "agent-2",
			MessageType: "text",
		},
		{
			Source:      "mqtt",
			SourceAgent: "agent-2",
			TargetAgent: "agent-1",
			MessageType: "command",
		},
		{
			Source:      "websocket",
			SourceAgent: "*",
			TargetAgent: "broadcast",
			MessageType: "status",
		},
	}

	for _, rule := range rules {
		err := router.RegisterRule(rule)
		if err != nil {
			t.Errorf("RegisterRule() error = %v", err)
		}
	}

	registeredRules := router.ListRules()
	if len(registeredRules) != 3 {
		t.Errorf("Rules count = %d, want 3", len(registeredRules))
	}
}

func TestRouter_ClearRules(t *testing.T) {
	router := NewRouter()

	// 添加一些规则
	router.RegisterRule(RouteRule{
		Source:      "websocket",
		SourceAgent: "agent-1",
		TargetAgent: "agent-2",
	})

	router.RegisterRule(RouteRule{
		Source:      "mqtt",
		SourceAgent: "agent-2",
		TargetAgent: "agent-1",
	})

	// 清除规则
	router.ClearRules()

	rules := router.ListRules()
	if len(rules) != 0 {
		t.Errorf("Rules count after clear = %d, want 0", len(rules))
	}
}

func TestRouter_Route(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*Router)
		msg       *protocol.GenericMessage
		wantErr   bool
		errMsg    string
		handledBy string
	}{
		{
			name: "matching websocket rule",
			setup: func(r *Router) {
				r.SetWebSocketHandler(func(msg protocol.Message) error {
					return nil
				})
				r.RegisterRule(RouteRule{
					Source:      "websocket",
					SourceAgent: "agent-1",
					TargetAgent: "agent-2",
					MessageType: "text",
				})
			},
			msg: &protocol.GenericMessage{
				MsgType: "text",
				Source:  "agent-1",
				Target:  "agent-2",
			},
			wantErr: false,
		},
		{
			name: "matching mqtt rule",
			setup: func(r *Router) {
				r.SetMQTTHandler(func(msg protocol.Message) error {
					return nil
				})
				r.RegisterRule(RouteRule{
					Source:      "mqtt",
					SourceAgent: "agent-2",
					TargetAgent: "agent-1",
					MessageType: "command",
				})
			},
			msg: &protocol.GenericMessage{
				MsgType: "command",
				Source:  "agent-2",
				Target:  "agent-1",
			},
			wantErr: false,
		},
		{
			name: "no matching rule",
			setup: func(r *Router) {
				r.RegisterRule(RouteRule{
					Source:      "websocket",
					SourceAgent: "agent-1",
					TargetAgent: "agent-2",
					MessageType: "text",
				})
			},
			msg: &protocol.GenericMessage{
				MsgType: "image",
				Source:  "agent-1",
				Target:  "agent-2",
			},
			wantErr: true,
			errMsg:  "no matching route found",
		},
		{
			name: "wildcard source agent",
			setup: func(r *Router) {
				r.SetWebSocketHandler(func(msg protocol.Message) error {
					return nil
				})
				r.RegisterRule(RouteRule{
					Source:      "websocket",
					SourceAgent: "",
					TargetAgent: "broadcast",
					MessageType: "status",
				})
			},
			msg: &protocol.GenericMessage{
				MsgType: "status",
				Source:  "any-agent",
				Target:  "broadcast",
			},
			wantErr: false,
		},
		{
			name: "wildcard message type",
			setup: func(r *Router) {
				r.SetMQTTHandler(func(msg protocol.Message) error {
					return nil
				})
				r.RegisterRule(RouteRule{
					Source:      "mqtt",
					SourceAgent: "agent-1",
					TargetAgent: "agent-2",
					MessageType: "",
				})
			},
			msg: &protocol.GenericMessage{
				MsgType: "anything",
				Source:  "agent-1",
				Target:  "agent-2",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter()
			tt.setup(router)

			err := router.Route(tt.msg)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Route() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("Route() error = %v, want %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Route() error = %v", err)
				}
			}
		})
	}
}

func TestRouter_Route_WithTransform(t *testing.T) {
	router := NewRouter()

	transformCalled := false
	router.SetWebSocketHandler(func(msg protocol.Message) error {
		return nil
	})

	router.RegisterRule(RouteRule{
		Source:      "websocket",
		SourceAgent: "agent-1",
		TargetAgent: "agent-2",
		MessageType: "text",
		Transform: func(msg protocol.Message) (protocol.Message, error) {
			transformCalled = true
			// 修改消息内容
			generic := msg.(*protocol.GenericMessage)
			generic.Metadata["transformed"] = "true"
			return generic, nil
		},
	})

	msg := &protocol.GenericMessage{
		MsgType:  "text",
		Source:   "agent-1",
		Target:   "agent-2",
		Metadata: make(map[string]string),
	}

	err := router.Route(msg)
	if err != nil {
		t.Errorf("Route() error = %v", err)
	}

	if !transformCalled {
		t.Error("Transform function was not called")
	}

	if msg.Metadata["transformed"] != "true" {
		t.Error("Message was not transformed")
	}
}

func TestRouter_Route_TransformError(t *testing.T) {
	router := NewRouter()

	router.SetWebSocketHandler(func(msg protocol.Message) error {
		return nil
	})

	transformErr := errors.New("transform error")
	router.RegisterRule(RouteRule{
		Source:      "websocket",
		SourceAgent: "agent-1",
		TargetAgent: "agent-2",
		MessageType: "text",
		Transform: func(msg protocol.Message) (protocol.Message, error) {
			return nil, transformErr
		},
	})

	msg := &protocol.GenericMessage{
		MsgType: "text",
		Source:  "agent-1",
		Target:  "agent-2",
	}

	err := router.Route(msg)
	if err == nil {
		t.Error("Route() should return error when transform fails")
	}

	if err.Error() != "transform message: transform error" {
		t.Errorf("Route() error = %v, want transform error", err)
	}
}

func TestRouter_Route_NoHandler(t *testing.T) {
	router := NewRouter()

	// 注册规则但不设置处理器
	router.RegisterRule(RouteRule{
		Source:      "websocket",
		SourceAgent: "agent-1",
		TargetAgent: "agent-2",
		MessageType: "text",
	})

	msg := &protocol.GenericMessage{
		MsgType: "text",
		Source:  "agent-1",
		Target:  "agent-2",
	}

	// 没有设置处理器，应该不会出错（只是不会路由）
	err := router.Route(msg)
	// 由于没有匹配的处理器，会返回 "no matching route found"
	if err == nil {
		t.Error("Route() should return error when no handler is set")
	}
}

func TestRouter_GetTargetTopic(t *testing.T) {
	router := NewRouter()

	topic := router.GetTargetTopic("agent-1", "agent-2", "text")
	expected := "agent/chat/agent-2/text"

	if topic != expected {
		t.Errorf("GetTargetTopic() = %v, want %v", topic, expected)
	}
}

func TestRouter_ListRules_Concurrency(t *testing.T) {
	router := NewRouter()

	// 添加多个规则
	for i := 0; i < 100; i++ {
		router.RegisterRule(RouteRule{
			Source:      "websocket",
			SourceAgent: "agent-1",
			TargetAgent: "agent-2",
			MessageType: "text",
		})
	}

	rules := router.ListRules()
	if len(rules) != 100 {
		t.Errorf("Rules count = %d, want 100", len(rules))
	}

	// 验证返回的副本是独立的
	rules[0].Source = "modified"

	rules2 := router.ListRules()
	if rules2[0].Source != "websocket" {
		t.Error("ListRules() should return a copy of rules")
	}
}

func TestRouter_ConcurrentAccess(t *testing.T) {
	router := NewRouter()

	// 并发注册规则
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				router.RegisterRule(RouteRule{
					Source:      "websocket",
					SourceAgent: "agent-1",
					TargetAgent: "agent-2",
					MessageType: "text",
				})
			}
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	rules := router.ListRules()
	if len(rules) != 100 {
		t.Errorf("Rules count = %d, want 100", len(rules))
	}
}

func TestRouter_SetHandlers(t *testing.T) {
	router := NewRouter()

	wsHandlerCalled := false
	router.SetWebSocketHandler(func(msg protocol.Message) error {
		wsHandlerCalled = true
		return nil
	})

	mqttHandlerCalled := false
	router.SetMQTTHandler(func(msg protocol.Message) error {
		mqttHandlerCalled = true
		return nil
	})

	// 注册规则
	router.RegisterRule(RouteRule{
		Source:      "websocket",
		SourceAgent: "agent-1",
		TargetAgent: "agent-2",
		MessageType: "text",
	})

	router.RegisterRule(RouteRule{
		Source:      "mqtt",
		SourceAgent: "agent-2",
		TargetAgent: "agent-1",
		MessageType: "command",
	})

	// 测试 WebSocket 路由
	wsMsg := &protocol.GenericMessage{
		MsgType: "text",
		Source:  "agent-1",
		Target:  "agent-2",
	}
	router.Route(wsMsg)

	if !wsHandlerCalled {
		t.Error("WebSocket handler was not called")
	}

	// 测试 MQTT 路由
	mqttMsg := &protocol.GenericMessage{
		MsgType: "command",
		Source:  "agent-2",
		Target:  "agent-1",
	}
	router.Route(mqttMsg)

	if !mqttHandlerCalled {
		t.Error("MQTT handler was not called")
	}
}

// 基准测试
func BenchmarkRouter_RegisterRule(b *testing.B) {
	router := NewRouter()
	rule := RouteRule{
		Source:      "websocket",
		SourceAgent: "agent-1",
		TargetAgent: "agent-2",
		MessageType: "text",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.RegisterRule(rule)
	}
}

func BenchmarkRouter_Route(b *testing.B) {
	router := NewRouter()
	router.SetWebSocketHandler(func(msg protocol.Message) error {
		return nil
	})

	// 添加一些规则
	for i := 0; i < 10; i++ {
		router.RegisterRule(RouteRule{
			Source:      "websocket",
			SourceAgent: "agent-1",
			TargetAgent: "agent-2",
			MessageType: "text",
		})
	}

	msg := &protocol.GenericMessage{
		MsgType: "text",
		Source:  "agent-1",
		Target:  "agent-2",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.Route(msg)
	}
}
