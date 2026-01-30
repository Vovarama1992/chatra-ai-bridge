package chatra

import "context"

type Sender string

const (
	SenderClient    Sender = "client"
	SenderSupporter Sender = "supporter"
	SenderAI        Sender = "ai"
)

type Message struct {
	ID          int64
	ChatID      string
	Sender      Sender
	Text        string
	ClientID    *string
	SupporterID *string
	CreatedAt   int64

	ClientInfo        map[string]any
	ClientIntegration map[string]any
}
type Outbound interface {
	SendToChat(ctx context.Context, chatID string, text string) error
	SendNote(ctx context.Context, chatID string, text string) error
}

// Repo — persistence
type Repo interface {
	SaveMessage(ctx context.Context, msg *Message) error
	GetHistory(ctx context.Context, chatID string) ([]Message, error)
}

// Service — оркестрация (без return)
type Service interface {
	HandleIncoming(ctx context.Context, msg *Message) error
	SaveOnly(ctx context.Context, msg *Message) error
}
