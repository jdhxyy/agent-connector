package router

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/jdhxyy/agent-connector/protocol"
)

func TestNewRouter(t *testing.T) {
	r := NewRouter()
	assert.NotNil(t, r)
	assert.Empty(t, r.ListRules())
}

func TestRouter_RegisterRule(t *testing.T) {
	r := NewRouter()

	rule := RouteRule{
		Source:      "websocket",
		SourceAgent: "agent-a",
		TargetAgent: "agent-b",
		MessageType: "text",
	}

	err := r.RegisterRule(rule)
	assert.NoError(t, err)

	rules := r.ListRules()
	assert.Len(t, rules, 1)
	assert.Equal(t, rule, rules[0])
}

func TestRouter_Route(t *testing.T) {
	r := NewRouter()

	var wsCalled, mqttCalled bool
	r.SetWebSocketHandler(func(msg protocol.Message) error {
		wsCalled = true
		return nil
	})
	r.SetMQTTHandler(func(msg protocol.Message) error {
		mqttCalled = true
		return nil
	})

	r.RegisterRule(RouteRule{
		Source:      "websocket",
		SourceAgent: "agent-a",
		TargetAgent: "agent-b",
	})

	msg := &protocol.GenericMessage{
		Source:  "agent-a",
		Target:  "agent-b",
		MsgType: "text",
	}

	err := r.Route(msg)
	assert.NoError(t, err)
	assert.True(t, mqttCalled)
	assert.False(t, wsCalled)
}

func TestRouter_Route_NoMatch(t *testing.T) {
	r := NewRouter()

	msg := &protocol.GenericMessage{
		Source:  "agent-c",
		Target:  "agent-d",
		MsgType: "text",
	}

	err := r.Route(msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no matching route found")
}

func TestRouter_Route_WithTransform(t *testing.T) {
	r := NewRouter()

	var receivedMsg protocol.Message
	r.SetMQTTHandler(func(msg protocol.Message) error {
		receivedMsg = msg
		return nil
	})

	r.RegisterRule(RouteRule{
		Source:      "websocket",
		SourceAgent: "agent-a",
		Transform: func(msg protocol.Message) (protocol.Message, error) {
			return &protocol.GenericMessage{
				ID:      "transformed",
				MsgType: msg.GetType(),
				Source:  msg.GetSource(),
				Target:  msg.GetTarget(),
			}, nil
		},
	})

	msg := &protocol.GenericMessage{
		ID:      "original",
		Source:  "agent-a",
		Target:  "agent-b",
		MsgType: "text",
	}

	err := r.Route(msg)
	assert.NoError(t, err)
	assert.Equal(t, "transformed", receivedMsg.GetID())
}

func TestRouter_Route_TransformError(t *testing.T) {
	r := NewRouter()

	r.RegisterRule(RouteRule{
		Source:      "websocket",
		SourceAgent: "agent-a",
		Transform: func(msg protocol.Message) (protocol.Message, error) {
			return nil, errors.New("transform error")
		},
	})

	msg := &protocol.GenericMessage{
		Source:  "agent-a",
		Target:  "agent-b",
		MsgType: "text",
	}

	err := r.Route(msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transform message")
}

func TestRouter_GetTargetTopic(t *testing.T) {
	r := NewRouter()

	topic := r.GetTargetTopic("agent-a", "agent-b", "text")
	assert.Equal(t, "agent/chat/agent-b/text", topic)
}

func TestRouter_ClearRules(t *testing.T) {
	r := NewRouter()

	r.RegisterRule(RouteRule{Source: "websocket"})
	r.RegisterRule(RouteRule{Source: "mqtt"})

	assert.Len(t, r.ListRules(), 2)

	r.ClearRules()
	assert.Empty(t, r.ListRules())
}

func TestRouter_MatchesRule(t *testing.T) {
	r := NewRouter()

	tests := []struct {
		name     string
		rule     RouteRule
		msg      protocol.Message
		expected bool
	}{
		{
			name:     "match all",
			rule:     RouteRule{},
			msg:      &protocol.GenericMessage{Source: "a", Target: "b", MsgType: "text"},
			expected: true,
		},
		{
			name:     "match source agent",
			rule:     RouteRule{SourceAgent: "agent-a"},
			msg:      &protocol.GenericMessage{Source: "agent-a"},
			expected: true,
		},
		{
			name:     "no match source agent",
			rule:     RouteRule{SourceAgent: "agent-a"},
			msg:      &protocol.GenericMessage{Source: "agent-b"},
			expected: false,
		},
		{
			name:     "match message type",
			rule:     RouteRule{MessageType: "text"},
			msg:      &protocol.GenericMessage{MsgType: "text"},
			expected: true,
		},
		{
			name:     "no match message type",
			rule:     RouteRule{MessageType: "text"},
			msg:      &protocol.GenericMessage{MsgType: "command"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.matchesRule(tt.msg, tt.rule)
			assert.Equal(t, tt.expected, result)
		})
	}
}
