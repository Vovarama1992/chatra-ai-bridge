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
}

// Repo — только persistence
type Repo interface {
	SaveMessage(ctx context.Context, msg *Message) error
	GetHistory(ctx context.Context, chatID string) ([]Message, error)
}

// Service — оркестрация + возврат ответа в Chatra
type Service interface {
	HandleIncoming(ctx context.Context, msg *Message) (string, error)
}
