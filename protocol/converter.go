package protocol

import (
	"encoding/json"
	"fmt"
	"time"
)

type Converter struct{}

func NewConverter() *Converter {
	return &Converter{}
}

func (c *Converter) PicoToGeneric(pico PicoMessage, source string) (*GenericMessage, error) {
	payload, err := json.Marshal(pico.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	return &GenericMessage{
		ID:        pico.ID,
		MsgType:   pico.Type,
		Source:    source,
		Payload:   payload,
		Timestamp: time.UnixMilli(pico.Timestamp),
		Metadata: map[string]string{
			"session_id": pico.SessionID,
		},
	}, nil
}

func (c *Converter) GenericToPico(msg *GenericMessage) (PicoMessage, error) {
	var payload map[string]any
	if len(msg.Payload) > 0 {
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return PicoMessage{}, fmt.Errorf("unmarshal payload: %w", err)
		}
	}

	return PicoMessage{
		Type:      msg.MsgType,
		ID:        msg.ID,
		Timestamp: msg.Timestamp.UnixMilli(),
		Payload:   payload,
	}, nil
}

func (c *Converter) MQTTToGeneric(topic string, payload []byte) (*GenericMessage, error) {
	var msg GenericMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		msg = GenericMessage{
			ID:        generateID(),
			MsgType:   "raw",
			Payload:   payload,
			Timestamp: time.Now(),
			Metadata: map[string]string{
				"topic": topic,
			},
		}
	}
	return &msg, nil
}

func (c *Converter) GenericToMQTT(msg *GenericMessage) (string, []byte, error) {
	payload, err := json.Marshal(msg)
	if err != nil {
		return "", nil, fmt.Errorf("marshal message: %w", err)
	}

	topic := msg.Metadata["topic"]
	if topic == "" {
		topic = fmt.Sprintf("agent/chat/%s/%s", msg.Target, msg.MsgType)
	}

	return topic, payload, nil
}

func generateID() string {
	return fmt.Sprintf("msg-%d", time.Now().UnixNano())
}
