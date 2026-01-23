package chatra

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/Vovarama1992/chatra-ai-bridge/internal/ai"
)

const confidenceThreshold = 2

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
	log.Printf("[svc] incoming chatId=%s text=%q", msg.ChatID, msg.Text)

	// 1) СОХРАНЯЕМ сообщение клиента СРАЗУ
	if err := s.repo.SaveMessage(ctx, msg); err != nil {
		log.Println("[svc] SaveMessage(client) error:", err)
		return err
	}

	// 2) грузим историю
	log.Println("[svc] load history")
	history, err := s.repo.GetHistory(ctx, msg.ChatID)
	if err != nil {
		log.Println("[svc] GetHistory error:", err)
		return err
	}
	log.Printf("[svc] history loaded: %d messages", len(history))

	// 3) контекст GPT
	techCtx := ""

	if len(msg.ClientIntegration) > 0 {
		b, _ := json.Marshal(msg.ClientIntegration)
		techCtx += "\n[CLIENT INTEGRATION DATA]\n" + string(b)
	}

	if len(msg.ClientInfo) > 0 {
		b, _ := json.Marshal(msg.ClientInfo)
		techCtx += "\n[CLIENT INFO]\n" + string(b)
	}

	aiHistory := []ai.Message{
		{
			Role: "system",
			Text: BaseSystemPrompt + "\n\n" + NotVPNDomainPrompt + techCtx,
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

	aiHistory = append(aiHistory, ai.Message{
		Role: "user",
		Text: msg.Text,
	})

	log.Printf("[svc] AI context size=%d", len(aiHistory))

	// 4) GPT
	ctxAI, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	raw, err := s.ai.GetReply(ctxAI, aiHistory)
	if err != nil {
		log.Println("[svc] GPT error:", err)
		return err
	}

	log.Println("[svc] GPT reply raw:", raw)

	// 5) JSON parse
	var resp aiResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		log.Println("[svc] AI unmarshal error:", err)
		return errors.New("invalid AI response format")
	}

	log.Printf("[svc] AI confidence=%.2f", resp.Confidence)

	// 6) low confidence
	if resp.Confidence < confidenceThreshold {
		note :=
			"[AI]\n" +
				"confidence: " + formatFloat(resp.Confidence) + "\n\n" +
				resp.Answer

		return s.outbound.SendNote(ctx, *msg.ClientID, note)
	}

	// 7) сохраняем AI
	if err := s.repo.SaveMessage(ctx, &Message{
		ChatID: msg.ChatID,
		Sender: SenderAI,
		Text:   resp.Answer,
	}); err != nil {
		log.Println("[svc] SaveMessage(ai) error:", err)
		return err
	}

	// 8) отправляем в Chatra
	return s.outbound.SendToChat(ctx, *msg.ClientID, resp.Answer)
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
