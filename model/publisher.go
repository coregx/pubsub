package model

import "time"

// Publisher represents a message publisher in the pub/sub system.
// Publishers are registered services or applications that can publish messages to topics.
//
// Each publisher is identified by a unique code and can be activated/deactivated.
// Inactive publishers cannot publish new messages.
type Publisher struct {
	ID          int64     `json:"id"`                        // Unique publisher ID
	Code        string    `json:"code" db:"publisher_code"`  // Unique publisher code (e.g., "user-service")
	Name        string    `json:"name"`                      // Human-readable publisher name
	Description string    `json:"description"`               // Publisher description
	IsActive    bool      `json:"isActive" db:"is_active"`   // Only active publishers can publish
	CreatedAt   time.Time `json:"createdAt" db:"created_at"` // Publisher registration time
}

// TableName returns the database table name for Publisher.
func (t Publisher) TableName() string {
	return tablePrefix + "publisher"
}

// NewPublisher creates a new active publisher.
//
// Parameters:
//   - code: Unique publisher identifier (e.g., "user-service", "order-processor")
//   - name: Human-readable name for display
//   - description: Purpose and details about this publisher
func NewPublisher(code, name, description string) Publisher {
	return Publisher{
		ID:          0,
		Code:        code,
		Name:        name,
		Description: description,
		IsActive:    true,
		CreatedAt:   time.Now(),
	}
}
