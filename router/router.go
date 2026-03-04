package router

import (
	"fmt"
	"sync"

	"github.com/jdhxyy/agent-connector/protocol"
)

type Router struct {
	rules      []RouteRule
	mu         sync.RWMutex
	wsHandler  func(protocol.Message) error
	mqttHandler func(protocol.Message) error
}

type RouteRule struct {
	Source      string
	SourceAgent string
	TargetAgent string
	MessageType string
	Transform   func(protocol.Message) (protocol.Message, error)
}

func NewRouter() *Router {
	return &Router{
		rules: make([]RouteRule, 0),
	}
}

func (r *Router) RegisterRule(rule RouteRule) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.rules = append(r.rules, rule)
	return nil
}

func (r *Router) Route(msg protocol.Message) error {
	r.mu.RLock()
	rules := make([]RouteRule, len(r.rules))
	copy(rules, r.rules)
	r.mu.RUnlock()

	for _, rule := range rules {
		if r.matchesRule(msg, rule) {
			transformedMsg := msg
			if rule.Transform != nil {
				var err error
				transformedMsg, err = rule.Transform(msg)
				if err != nil {
					return fmt.Errorf("transform message: %w", err)
				}
			}

			switch rule.Source {
			case "websocket":
				if r.mqttHandler != nil {
					return r.mqttHandler(transformedMsg)
				}
			case "mqtt":
				if r.wsHandler != nil {
					return r.wsHandler(transformedMsg)
				}
			}
		}
	}

	return fmt.Errorf("no matching route found")
}

func (r *Router) matchesRule(msg protocol.Message, rule RouteRule) bool {
	if rule.SourceAgent != "" && msg.GetSource() != rule.SourceAgent {
		return false
	}
	if rule.TargetAgent != "" && msg.GetTarget() != rule.TargetAgent {
		return false
	}
	if rule.MessageType != "" && msg.GetType() != rule.MessageType {
		return false
	}
	return true
}

func (r *Router) SetWebSocketHandler(handler func(protocol.Message) error) {
	r.wsHandler = handler
}

func (r *Router) SetMQTTHandler(handler func(protocol.Message) error) {
	r.mqttHandler = handler
}

func (r *Router) GetTargetTopic(sourceAgent, targetAgent, msgType string) string {
	return fmt.Sprintf("agent/chat/%s/%s", targetAgent, msgType)
}

func (r *Router) ListRules() []RouteRule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rules := make([]RouteRule, len(r.rules))
	copy(rules, r.rules)
	return rules
}

func (r *Router) ClearRules() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.rules = make([]RouteRule, 0)
}
