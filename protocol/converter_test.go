package protocol

import (
	"testing"
	"time"
)

func TestConverter_MQTTToPico(t *testing.T) {
	converter := NewConverter()

	tests := []struct {
		name        string
		topic       string
		data        []byte
		wantType    string
		wantSession string
		wantContent string
		wantErr     bool
	}{
		{
			name:        "agent message",
			topic:       "agent/ext-agent-x/message",
			data:        []byte("Hello from external agent!"),
			wantType:    TypeMessageSend,
			wantSession: "agent:ext-agent-x",
			wantContent: "Hello from external agent!",
			wantErr:     false,
		},
		{
			name:        "group message",
			topic:       "group/ext-group-team1/message",
			data:        []byte("Hello team!"),
			wantType:    TypeMessageSend,
			wantSession: "group:ext-group-team1",
			wantContent: "Hello team!",
			wantErr:     false,
		},
		{
			name:        "broadcast message",
			topic:       "broadcast/message",
			data:        []byte("Broadcast to all!"),
			wantType:    TypeMessageSend,
			wantSession: "broadcast:all",
			wantContent: "Broadcast to all!",
			wantErr:     false,
		},
		{
			name:    "invalid topic",
			topic:   "invalid/topic",
			data:    []byte("test"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pico, err := converter.MQTTToPico(tt.topic, tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("MQTTToPico() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if pico.Type != tt.wantType {
				t.Errorf("MQTTToPico() Type = %v, want %v", pico.Type, tt.wantType)
			}
			if pico.SessionID != tt.wantSession {
				t.Errorf("MQTTToPico() SessionID = %v, want %v", pico.SessionID, tt.wantSession)
			}
			if pico.GetContent() != tt.wantContent {
				t.Errorf("MQTTToPico() Content = %v, want %v", pico.GetContent(), tt.wantContent)
			}
		})
	}
}

func TestConverter_PicoToMQTT(t *testing.T) {
	converter := NewConverter()

	tests := []struct {
		name        string
		pico        PicoMessage
		wantTopic   string
		wantContent string
		wantErr     bool
	}{
		{
			name: "agent message",
			pico: PicoMessage{
				Type:      TypeMessageSend,
				SessionID: "agent:ext-agent-x",
				Payload:   map[string]any{"content": "Hello external agent!"},
			},
			wantTopic:   "agent/ext-agent-x/message",
			wantContent: "Hello external agent!",
			wantErr:     false,
		},
		{
			name: "group message",
			pico: PicoMessage{
				Type:      TypeMessageSend,
				SessionID: "group:ext-group-team1",
				Payload:   map[string]any{"content": "Hello team!"},
			},
			wantTopic:   "group/ext-group-team1/message",
			wantContent: "Hello team!",
			wantErr:     false,
		},
		{
			name: "broadcast message",
			pico: PicoMessage{
				Type:      TypeMessageSend,
				SessionID: "broadcast:all",
				Payload:   map[string]any{"content": "Broadcast message!"},
			},
			wantTopic:   "broadcast/message",
			wantContent: "Broadcast message!",
			wantErr:     false,
		},
		{
			name: "invalid session_id",
			pico: PicoMessage{
				Type:      TypeMessageSend,
				SessionID: "invalid-session",
				Payload:   map[string]any{"content": "test"},
			},
			wantErr: true,
		},
		{
			name: "missing content",
			pico: PicoMessage{
				Type:      TypeMessageSend,
				SessionID: "agent:ext-agent-x",
				Payload:   map[string]any{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topic, data, err := converter.PicoToMQTT(tt.pico)
			if (err != nil) != tt.wantErr {
				t.Errorf("PicoToMQTT() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if topic != tt.wantTopic {
				t.Errorf("PicoToMQTT() topic = %v, want %v", topic, tt.wantTopic)
			}
			if string(data) != tt.wantContent {
				t.Errorf("PicoToMQTT() content = %v, want %v", string(data), tt.wantContent)
			}
		})
	}
}

func TestConverter_PicoToGeneric(t *testing.T) {
	converter := NewConverter()

	pico := PicoMessage{
		Type:      TypeMessageSend,
		ID:        "msg-123",
		SessionID: "agent:ext-agent-x",
		Timestamp: 1234567890000,
		Payload:   map[string]any{"content": "Hello!"},
	}

	generic, err := converter.PicoToGeneric(pico, "local-agent")
	if err != nil {
		t.Fatalf("PicoToGeneric() error = %v", err)
	}

	if generic.MsgType != TypeMessageSend {
		t.Errorf("MsgType = %v, want %v", generic.MsgType, TypeMessageSend)
	}
	if generic.ID != "msg-123" {
		t.Errorf("ID = %v, want %v", generic.ID, "msg-123")
	}
	if generic.Source != "local-agent" {
		t.Errorf("Source = %v, want %v", generic.Source, "local-agent")
	}
	if generic.Metadata["session_id"] != "agent:ext-agent-x" {
		t.Errorf("Metadata[session_id] = %v, want %v", generic.Metadata["session_id"], "agent:ext-agent-x")
	}
}

func TestConverter_GenericToPico(t *testing.T) {
	converter := NewConverter()

	generic := &GenericMessage{
		ID:        "msg-123",
		MsgType:   TypeMessageSend,
		Timestamp: time.Now(),
		Metadata: map[string]string{
			"session_id": "agent:ext-agent-x",
		},
		Payload: []byte(`{"content":"Hello!"}`),
	}

	pico, err := converter.GenericToPico(generic)
	if err != nil {
		t.Fatalf("GenericToPico() error = %v", err)
	}

	if pico.Type != TypeMessageSend {
		t.Errorf("Type = %v, want %v", pico.Type, TypeMessageSend)
	}
	if pico.ID != "msg-123" {
		t.Errorf("ID = %v, want %v", pico.ID, "msg-123")
	}
	if pico.SessionID != "agent:ext-agent-x" {
		t.Errorf("SessionID = %v, want %v", pico.SessionID, "agent:ext-agent-x")
	}
	if pico.GetContent() != "Hello!" {
		t.Errorf("Content = %v, want %v", pico.GetContent(), "Hello!")
	}
}

func TestBuildAgentTopic(t *testing.T) {
	tests := []struct {
		agentID  string
		expected string
	}{
		{"ext-agent-x", "agent/ext-agent-x/message"},
		{"agent-123", "agent/agent-123/message"},
	}

	for _, tt := range tests {
		result := BuildAgentTopic(tt.agentID)
		if result != tt.expected {
			t.Errorf("BuildAgentTopic(%q) = %q, want %q", tt.agentID, result, tt.expected)
		}
	}
}

func TestBuildGroupTopic(t *testing.T) {
	tests := []struct {
		groupID  string
		expected string
	}{
		{"ext-group-team1", "group/ext-group-team1/message"},
		{"group-123", "group/group-123/message"},
	}

	for _, tt := range tests {
		result := BuildGroupTopic(tt.groupID)
		if result != tt.expected {
			t.Errorf("BuildGroupTopic(%q) = %q, want %q", tt.groupID, result, tt.expected)
		}
	}
}

func TestBuildBroadcastTopic(t *testing.T) {
	result := BuildBroadcastTopic()
	expected := "broadcast/message"
	if result != expected {
		t.Errorf("BuildBroadcastTopic() = %q, want %q", result, expected)
	}
}


