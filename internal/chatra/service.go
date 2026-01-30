package chatra

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"time"

	"github.com/Vovarama1992/chatra-ai-bridge/internal/ai"
)

const confidenceThreshold = 1.3

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
	Reason     string  `json:"reason"`
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

	// 3) готовим base + cases отдельно
	basePrompt := BaseSystemPrompt
	domainCases := NotVPNDomainPrompt

	var clientInfo string
	if len(msg.ClientInfo) > 0 {
		b, _ := json.Marshal(msg.ClientInfo)
		clientInfo = string(b)
	}

	var integrationData string
	if len(msg.ClientIntegration) > 0 {
		b, _ := json.Marshal(msg.ClientIntegration)
		integrationData = string(b)
	}

	// История в роли user/assistant БЕЗ system
	aiHistory := make([]ai.Message, 0, len(history))
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
	log.Printf("[svc] history for AI: %d messages", len(aiHistory))

	// 4) GPT
	ctxAI, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	raw, err := s.ai.GetReply(
		ctxAI,
		basePrompt,
		domainCases,
		clientInfo,
		integrationData,
		aiHistory,
		msg.Text,
	)
	if err != nil {
		log.Println("[svc] GPT error:", err)
		return err
	}

	log.Printf("[svc] AI raw=%s", raw)

	// 5) JSON parse
	var resp aiResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		log.Println("[svc] AI unmarshal error:", err)
		return errors.New("invalid AI response format")
	}

	log.Printf("[svc] AI answer=%q", resp.Answer)
	log.Printf("[svc] AI confidence=%.2f", resp.Confidence)
	log.Printf("[svc] AI reason=%q", resp.Reason)

	// 6) low confidence -> note for operator
	if resp.Confidence < confidenceThreshold {
		note :=
			"[AI]\n" +
				"confidence: " + formatFloat(resp.Confidence) + "\n" +
				"reason: " + resp.Reason + "\n\n" +
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

	// 8) отправляем в Chatra (видит клиент)
	return s.outbound.SendToChat(ctx, *msg.ClientID, resp.Answer)
}

func (s *service) SaveOnly(ctx context.Context, msg *Message) error {
	log.Printf("[svc] save only chatId=%s sender=%s text=%q",
		msg.ChatID, msg.Sender, msg.Text,
	)
	return s.repo.SaveMessage(ctx, msg)
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}
