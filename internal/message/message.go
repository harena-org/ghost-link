package message

import (
	"encoding/json"
	"fmt"
)

// Envelope is the structured JSON format for agent-to-agent messages.
type Envelope struct {
	V       int               `json:"v"`
	Type    string            `json:"type"`
	Body    string            `json:"body"`
	ReplyTo string            `json:"reply_to,omitempty"`
	Meta    map[string]string `json:"meta,omitempty"`
}

// Encode marshals an Envelope to JSON bytes.
func Encode(env *Envelope) ([]byte, error) {
	data, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("encode envelope: %w", err)
	}
	return data, nil
}

// Decode attempts to unmarshal data as an Envelope. Returns nil, nil if the
// data is not a valid envelope (e.g., legacy raw text messages).
func Decode(data []byte) (*Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, nil
	}
	// Must have version field to be a valid envelope
	if env.V == 0 {
		return nil, nil
	}
	return &env, nil
}
