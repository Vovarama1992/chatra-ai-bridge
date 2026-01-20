package chatra

import (
	"context"
	"encoding/json"
	"errors"
	"log"
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
	log.Printf("[svc] incoming chatId=%s text=%q\n", msg.ChatID, msg.Text)

	log.Println("[svc] load history")
	history, err := s.repo.GetHistory(ctx, msg.ChatID)
	if err != nil {
		log.Println("[svc] GetHistory error:", err)
		return err
	}
	log.Printf("[svc] history loaded: %d messages\n", len(history))

	log.Println("[svc] build AI context")
	aiHistory := []ai.Message{
		{
			Role: "system",
			Text: BaseSystemPrompt + "\n\n" + NotVPNDomainPrompt,
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

	log.Printf("[svc] AI context size=%d\n", len(aiHistory))

	ctxAI, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	log.Println("[svc] call GPT")
	raw, err := s.ai.GetReply(ctxAI, aiHistory)
	if err != nil {
		log.Println("[svc] GPT error:", err)
		return err
	}
	log.Println("[svc] GPT reply received")

	var resp aiResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		log.Println("[svc] AI unmarshal error:", err)
		return errors.New("invalid AI response format")
	}

	log.Printf("[svc] AI confidence=%.3f\n", resp.Confidence)

	log.Println("[svc] save client message")
	if err := s.repo.SaveMessage(ctx, msg); err != nil {
		log.Println("[svc] SaveMessage(client) error:", err)
		return err
	}

	if resp.Confidence < confidenceThreshold {
		log.Println("[svc] low confidence -> send note")

		note :=
			"[AI, low confidence]\n" +
				"confidence=" + formatFloat(resp.Confidence) + "\n\n" +
				resp.Answer

		return s.outbound.SendNote(ctx, msg.ChatID, note)
	}

	log.Println("[svc] save AI message")
	if err := s.repo.SaveMessage(ctx, &Message{
		ChatID: msg.ChatID,
		Sender: SenderAI,
		Text:   resp.Answer,
	}); err != nil {
		log.Println("[svc] SaveMessage(ai) error:", err)
		return err
	}

	log.Println("[svc] send message to Chatra")
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
