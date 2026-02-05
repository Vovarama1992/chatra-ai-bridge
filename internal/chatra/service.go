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

var allowedModes = map[string]bool{
	// "CLIENT_ONLY": true,
	// "CASES_USED":  true,
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
	Answer string `json:"answer"`
	Mode   string `json:"mode"`
	Reason string `json:"reason"`
}

func (s *service) HandleIncoming(ctx context.Context, msg *Message) error {
	log.Printf("\n========== NEW MESSAGE ==========")
	log.Printf("[svc] chatId=%s text=%q", msg.ChatID, msg.Text)

	// 1) save client message
	if err := s.repo.SaveMessage(ctx, msg); err != nil {
		log.Println("[svc] SaveMessage(client) error:", err)
		return err
	}

	// 2) load history
	history, err := s.repo.GetHistory(ctx, msg.ChatID)
	if err != nil {
		log.Println("[svc] GetHistory error:", err)
		return err
	}

	// 3) prepare data
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

	// ===== PHASE 1 — CLIENT INFO ONLY =====
	log.Println("----- PHASE 1: CLIENT_INFO_ONLY -----")

	respCI, err := s.checkClientInfo(
		ctx,
		aiHistory,
		msg.Text,
		clientInfo,
		integrationData,
	)

	if err != nil {
		log.Println("[phase1] error:", err)
	} else {
		log.Printf("[phase1] mode=%s reason=%s", respCI.Mode, respCI.Reason)
	}

	if err == nil && respCI.Mode == "CLIENT_ONLY" {
		log.Println("[phase1] SUCCESS -> answer from client data")

		_ = s.repo.SaveMessage(ctx, &Message{
			ChatID: msg.ChatID,
			Sender: SenderAI,
			Text:   respCI.Answer,
		})
		return s.outbound.SendToChat(ctx, *msg.ClientID, respCI.Answer)
	}

	log.Println("[phase1] NOT ENOUGH -> switching to CASES")

	// ===== PHASE 2 — CASES =====
	log.Println("----- PHASE 2: CASES -----")

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
		log.Println("[phase2] GPT error:", err)
		return err
	}

	var resp aiResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		log.Println("[phase2] JSON error:", err)
		return errors.New("invalid AI response format")
	}

	log.Printf("[phase2] mode=%s reason=%s", resp.Mode, resp.Reason)

	if !allowedModes[resp.Mode] {
		log.Println("[phase2] BLOCKED -> sending to operator")

		note :=
			"[AI]\n" +
				"mode: " + resp.Mode + "\n" +
				"reason: " + resp.Reason + "\n\n" +
				resp.Answer

		return s.outbound.SendNote(ctx, *msg.ClientID, note)
	}

	log.Println("[phase2] ALLOWED -> sending to client")

	if err := s.repo.SaveMessage(ctx, &Message{
		ChatID: msg.ChatID,
		Sender: SenderAI,
		Text:   resp.Answer,
	}); err != nil {
		log.Println("[svc] SaveMessage(ai) error:", err)
		return err
	}

	return s.outbound.SendToChat(ctx, *msg.ClientID, resp.Answer)
}

func (s *service) checkClientInfo(
	ctx context.Context,
	history []ai.Message,
	lastUserText string,
	clientInfo string,
	integrationData string,
) (aiResponse, error) {

	raw, err := s.ai.GetReply(
		ctx,
		ClientInfoOnlyPrompt,
		"",
		clientInfo,
		integrationData,
		history,
		lastUserText,
	)
	if err != nil {
		return aiResponse{}, err
	}

	var resp aiResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return aiResponse{}, err
	}

	return resp, nil
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
