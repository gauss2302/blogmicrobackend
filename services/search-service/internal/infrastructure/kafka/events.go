package kafka

import "encoding/json"

// Search index event contract: entity_type, event_type, entity_id, payload, timestamp, message_id.

type IndexEvent struct {
	EntityType string          `json:"entity_type"` // "user" | "post"
	EventType  string          `json:"event_type"`  // "created" | "updated" | "deleted"
	EntityID   string          `json:"entity_id"`
	Payload    json.RawMessage `json:"payload"`
	Timestamp  string          `json:"timestamp"`  // ISO8601
	MessageID  string          `json:"message_id"`
}

type UserPayload struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
	Bio     string `json:"bio"`
}

type PostPayload struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Title     string `json:"title"`
	Slug      string `json:"slug"`
	Content   string `json:"content"`
	Published bool   `json:"published"`
}
