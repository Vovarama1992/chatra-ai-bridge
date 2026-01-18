package chatra

import (
	"context"
	"time"

	"github.com/Vovarama1992/chatra-ai-bridge/internal/ai"
)

type service struct {
	repo Repo
	ai   ai.AI
}

func NewService(repo Repo, aiClient ai.AI) Service {
	return &service{
		repo: repo,
		ai:   aiClient,
	}
}

// HandleIncoming
// 1) сохраняем входящее сообщение
// 2) берём историю
// 3) добавляем system-prompt
// 4) вызываем AI
// 5) сохраняем ответ AI
// 6) возвращаем текст ответа (для Chatra)
func (s *service) HandleIncoming(ctx context.Context, msg *Message) (string, error) {
	// 1) save incoming
	if err := s.repo.SaveMessage(ctx, msg); err != nil {
		return "", err
	}

	// 2) load history
	history, err := s.repo.GetHistory(ctx, msg.ChatID)
	if err != nil {
		return "", err
	}

	// 3) map to AI messages
	aiHistory := make([]ai.Message, 0, len(history)+1)

	// system prompt (HARDCODE, позже вынесем)
	aiHistory = append(aiHistory, ai.Message{
		Role: "system",
		Text: "Ты ассистент службы поддержки. Отвечай кратко, по делу, вежливо.",
	})

	for _, m := range history {
		role := "user"
		switch m.Sender {
		case SenderAI, SenderSupporter:
			role = "assistant"
		case SenderClient:
			role = "user"
		}

		aiHistory = append(aiHistory, ai.Message{
			Role: role,
			Text: m.Text,
		})
	}

	// 4) call AI
	ctxAI, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	reply, err := s.ai.GetReply(ctxAI, aiHistory)
	if err != nil {
		return "", err
	}

	// 5) save AI reply
	if err := s.repo.SaveMessage(ctx, &Message{
		ChatID: msg.ChatID,
		Sender: SenderAI,
		Text:   reply,
	}); err != nil {
		return "", err
	}

	// 6) return to handler → Chatra
	return reply, nil
}
