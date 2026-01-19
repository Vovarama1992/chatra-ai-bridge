package chatra

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Vovarama1992/chatra-ai-bridge/internal/ai"
)

const confidenceThreshold = 0.7

type service struct {
	repo     Repo
	ai       ai.AI
	outbound Outbound
}

func NewService(repo Repo, aiClient ai.AI, outbound Outbound) Service {
	return &service{
		repo:     repo,
		ai:       aiClient,
		outbound: outbound,
	}
}

type aiResponse struct {
	Answer     string  `json:"answer"`
	Confidence float64 `json:"confidence"`
}

func (s *service) HandleIncoming(ctx context.Context, msg *Message) error {
	// 1) подгружаем ИСТОРИЮ (только прошлые сообщения)
	history, err := s.repo.GetHistory(ctx, msg.ChatID)
	if err != nil {
		return err
	}

	// 2) собираем GPT-контекст
	aiHistory := []ai.Message{
		{
			Role: "system",
			Text: BaseSystemPrompt + "\n\n" + NotVPNDomainPrompt,
		},
	}

	// 3) прошлые сообщения
	for _, m := range history {
		role := "user"
		if m.Sender == SenderAI || m.Sender == SenderSupporter {
			role = "assistant"
		}
		aiHistory = append(aiHistory, ai.Message{
			Role: role,
			Text: m.Text,
		})
	}

	// 4) ТЕКУЩЕЕ сообщение пользователя (отдельно, последним)
	aiHistory = append(aiHistory, ai.Message{
		Role: "user",
		Text: msg.Text,
	})

	// 5) запрос в GPT
	ctxAI, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	raw, err := s.ai.GetReply(ctxAI, aiHistory)
	if err != nil {
		return err
	}

	var resp aiResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return errors.New("invalid AI response format")
	}

	// 6) сохраняем входящее сообщение пользователя
	if err := s.repo.SaveMessage(ctx, msg); err != nil {
		return err
	}

	// 7) low confidence → внутренняя заметка оператору
	if resp.Confidence < confidenceThreshold {
		note :=
			"[AI, low confidence]\n" +
				"confidence=" + formatFloat(resp.Confidence) + "\n\n" +
				resp.Answer

		return s.outbound.SendNote(ctx, msg.ChatID, note)
	}

	// 8) high confidence → сохраняем ответ AI
	if err := s.repo.SaveMessage(ctx, &Message{
		ChatID: msg.ChatID,
		Sender: SenderAI,
		Text:   resp.Answer,
	}); err != nil {
		return err
	}

	// 9) отправляем клиенту
	return s.outbound.SendToChat(ctx, msg.ChatID, resp.Answer)
}

// локально, чтобы не тащить fmt в hot path
func formatFloat(v float64) string {
	switch {
	case v >= 1:
		return "1.0"
	case v <= 0:
		return "0.0"
	default:
		return string([]byte{'0' + byte(v*10)})
	}
}
