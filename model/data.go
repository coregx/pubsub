// Package model contains all domain models and data structures for the PubSub system.
package model

import "time"

// Attributes represents a map of key-value pairs for message metadata.
type Attributes map[string]string

// DataMessage represents a message with metadata for delivery to subscribers.
type DataMessage struct {
	MessageID   string     `json:"messageID"`
	PublishTime time.Time  `json:"publishTime"`
	OrderingKey string     `json:"orderingKey"`
	Attributes  Attributes `json:"attributes"`
	Data        string     `json:"data"`
	Identifier  string
}

// NewDataMessage creates a new DataMessage with the given parameters.
func NewDataMessage(messageID string, _ time.Time, identifier, data string) *DataMessage {
	options := make(map[string]string)
	options["publisher"] = "wagon"
	options["version"] = "1.0"

	return &DataMessage{
		Attributes: options,
		MessageID:  messageID,
		Data:       data,
		Identifier: identifier,
	}
}

// ToString returns the message data as a string.
func (d *DataMessage) ToString() string {
	return d.Data
}

// FromString parses message data from a string (currently a no-op).
func (d *DataMessage) FromString(_ string) error {
	return nil
}

// ToBase64 returns the message data as a base64 string (currently returns empty).
func (d *DataMessage) ToBase64() string {
	return ""
}
