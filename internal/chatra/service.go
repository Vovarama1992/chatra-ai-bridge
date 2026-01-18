package chatra

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Vovarama1992/chatra-ai-bridge/internal/ai"
)

const confidenceThreshold = 0.7

type Outbound interface {
	SendToChat(ctx context.Context, chatID string, text string) error
	SendToAdmin(ctx context.Context, adminID string, text string) error
}

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
	// 1) save incoming
	if err := s.repo.SaveMessage(ctx, msg); err != nil {
		return err
	}

	// 2) history
	history, err := s.repo.GetHistory(ctx, msg.ChatID)
	if err != nil {
		return err
	}

	// 3) prompt + history
	aiHistory := []ai.Message{
		{
			Role: "system",
			Text: `Ты ассистент службы поддержки.

Твоя задача:
1) Дать лучший возможный ответ пользователю.
2) Оценить УВЕРЕННОСТЬ (confidence), можно ли отвечать без оператора.

Что такое confidence:
- 1.0 — полностью уверен, стандартный вопрос.
- 0.7–0.9 — почти уверен, возможны нюансы.
- 0.4–0.6 — есть сомнения, лучше оператор.
- < 0.4 — нужен человек.

Правила:
- confidence — число от 0 до 1.
- Не завышай confidence.
- Деньги, возвраты, аккаунты, юридические вопросы → низкий confidence.

Верни СТРОГО JSON:
{
  "answer": "ответ пользователю",
  "confidence": 0.0
}`,
		},
	}

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

	// 4) GPT
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

	// 5) low confidence → admin
	if resp.Confidence < confidenceThreshold {
		if msg.SupporterID == nil {
			return errors.New("supporter_id required for low confidence flow")
		}
		return s.outbound.SendToAdmin(ctx, *msg.SupporterID, resp.Answer)
	}

	// 6) high confidence → save + client
	if err := s.repo.SaveMessage(ctx, &Message{
		ChatID: msg.ChatID,
		Sender: SenderAI,
		Text:   resp.Answer,
	}); err != nil {
		return err
	}

	return s.outbound.SendToChat(ctx, msg.ChatID, resp.Answer)
}
