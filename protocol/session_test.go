package protocol

import "testing"

func TestTopicToSessionID(t *testing.T) {
	tests := []struct {
		name     string
		topic    string
		expected string
	}{
		{
			name:     "agent topic",
			topic:    "agent/ext-agent-x/message",
			expected: "agent:ext-agent-x",
		},
		{
			name:     "group topic",
			topic:    "group/ext-group-team1/message",
			expected: "group:ext-group-team1",
		},
		{
			name:     "broadcast topic",
			topic:    "broadcast/message",
			expected: "broadcast:all",
		},
		{
			name:     "invalid topic - too short",
			topic:    "agent",
			expected: "",
		},
		{
			name:     "invalid topic - unknown prefix",
			topic:    "unknown/something/message",
			expected: "",
		},
		{
			name:     "empty topic",
			topic:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TopicToSessionID(tt.topic)
			if result != tt.expected {
				t.Errorf("TopicToSessionID(%q) = %q, want %q", tt.topic, result, tt.expected)
			}
		})
	}
}

func TestParseSessionID(t *testing.T) {
	tests := []struct {
		name           string
		sessionID      string
		expectedType   string
		expectedID     string
	}{
		{
			name:         "agent session",
			sessionID:    "agent:ext-agent-x",
			expectedType: "agent",
			expectedID:   "ext-agent-x",
		},
		{
			name:         "group session",
			sessionID:    "group:ext-group-team1",
			expectedType: "group",
			expectedID:   "ext-group-team1",
		},
		{
			name:         "broadcast session",
			sessionID:    "broadcast:all",
			expectedType: "broadcast",
			expectedID:   "all",
		},
		{
			name:         "invalid session - no colon",
			sessionID:    "invalidsession",
			expectedType: "",
			expectedID:   "",
		},
		{
			name:         "empty session",
			sessionID:    "",
			expectedType: "",
			expectedID:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetType, targetID := ParseSessionID(tt.sessionID)
			if targetType != tt.expectedType {
				t.Errorf("ParseSessionID(%q) type = %q, want %q", tt.sessionID, targetType, tt.expectedType)
			}
			if targetID != tt.expectedID {
				t.Errorf("ParseSessionID(%q) id = %q, want %q", tt.sessionID, targetID, tt.expectedID)
			}
		})
	}
}

func TestNewSessionID(t *testing.T) {
	tests := []struct {
		name       string
		targetType string
		targetID   string
		expected   string
	}{
		{
			name:       "agent session",
			targetType: "agent",
			targetID:   "ext-agent-x",
			expected:   "agent:ext-agent-x",
		},
		{
			name:       "group session",
			targetType: "group",
			targetID:   "ext-group-team1",
			expected:   "group:ext-group-team1",
		},
		{
			name:       "broadcast session",
			targetType: "broadcast",
			targetID:   "all",
			expected:   "broadcast:all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewSessionID(tt.targetType, tt.targetID)
			if result != tt.expected {
				t.Errorf("NewSessionID(%q, %q) = %q, want %q", tt.targetType, tt.targetID, result, tt.expected)
			}
		})
	}
}

func TestIsAgentSession(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		expected  bool
	}{
		{"agent session", "agent:ext-agent-x", true},
		{"group session", "group:ext-group-team1", false},
		{"broadcast session", "broadcast:all", false},
		{"invalid session", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAgentSession(tt.sessionID)
			if result != tt.expected {
				t.Errorf("IsAgentSession(%q) = %v, want %v", tt.sessionID, result, tt.expected)
			}
		})
	}
}

func TestIsGroupSession(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		expected  bool
	}{
		{"agent session", "agent:ext-agent-x", false},
		{"group session", "group:ext-group-team1", true},
		{"broadcast session", "broadcast:all", false},
		{"invalid session", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsGroupSession(tt.sessionID)
			if result != tt.expected {
				t.Errorf("IsGroupSession(%q) = %v, want %v", tt.sessionID, result, tt.expected)
			}
		})
	}
}

func TestIsBroadcastSession(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		expected  bool
	}{
		{"agent session", "agent:ext-agent-x", false},
		{"group session", "group:ext-group-team1", false},
		{"broadcast session", "broadcast:all", true},
		{"invalid session", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsBroadcastSession(tt.sessionID)
			if result != tt.expected {
				t.Errorf("IsBroadcastSession(%q) = %v, want %v", tt.sessionID, result, tt.expected)
			}
		})
	}
}
