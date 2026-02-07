package chatra

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/Vovarama1992/chatra-ai-bridge/internal/ai"
)

type service struct {
	repo     Repo
	ai       ai.AI
	outbound Outbound
}

var allowedModes = map[string]bool{}

func NewService(repo Repo, aiClient ai.AI, outbound Outbound) Service {
	return &service{
		repo:     repo,
		ai:       aiClient,
		outbound: outbound,
	}
}

type aiFacts struct {
	Facts []string `json:"facts"`
	Mode  string   `json:"mode"`
}

type aiAnswer struct {
	Answer string   `json:"answer"`
	Facts  []string `json:"facts"`
	Mode   string   `json:"mode"`
}

func (s *service) HandleIncoming(ctx context.Context, msg *Message) error {
	log.Println("========== NEW MESSAGE ==========")
	log.Printf("[svc] chatId=%s text=%q", msg.ChatID, msg.Text)

	_ = s.repo.SaveMessage(ctx, msg)
	history, _ := s.repo.GetHistory(ctx, msg.ChatID)

	aiHistory := make([]ai.Message, 0, len(history))
	for _, m := range history {
		role := "user"
		if m.Sender == SenderAI || m.Sender == SenderSupporter {
			role = "assistant"
		}
		aiHistory = append(aiHistory, ai.Message{Role: role, Text: m.Text})
	}

	clientInfo, _ := json.Marshal(msg.ClientInfo)
	integrationData, _ := json.Marshal(msg.ClientIntegration)

	// STEP 1 — FACT SELECTOR
	factsResp, _ := s.selectFacts(
		ctx,
		aiHistory,
		msg.Text,
		string(clientInfo),
		string(integrationData),
	)

	s.logStage("FACT_SELECTOR", factsResp)

	if factsResp.Mode == "NEED_OPERATOR" {
		return s.sendFullNote(ctx, msg, "FACT_SELECTOR", factsResp, "", "")
	}

	// STEP 2 — FACT VALIDATOR
	ok, _ := s.validateFacts(ctx, aiHistory, msg.Text, factsResp.Facts)
	s.logStage("FACT_VALIDATOR", ok)

	if !ok {
		return s.sendFullNote(ctx, msg, "FACT_VALIDATOR", factsResp, "", "")
	}

	// STEP 3 — ANSWER BUILDER
	answerResp, _ := s.buildAnswer(
		ctx,
		aiHistory,
		msg.Text,
		factsResp.Facts,
	)

	s.logStage("ANSWER_BUILDER", answerResp)

	if answerResp.Mode == "NEED_OPERATOR" {
		return s.sendFullNote(ctx, msg, "ANSWER_BUILDER", factsResp, answerResp.Answer, answerResp.Mode)
	}

	// STEP 4 — ANSWER VALIDATOR
	ok, _ = s.validateAnswer(ctx, msg.Text, answerResp.Answer, answerResp.Facts)
	s.logStage("ANSWER_VALIDATOR", ok)

	if !ok {
		return s.sendFullNote(ctx, msg, "ANSWER_VALIDATOR", factsResp, answerResp.Answer, answerResp.Mode)
	}

	// ---------- ФИНАЛЬНОЕ РЕШЕНИЕ ПО MODE ----------
	if allowedModes[answerResp.Mode] {
		_ = s.repo.SaveMessage(ctx, &Message{
			ChatID: msg.ChatID,
			Sender: SenderAI,
			Text:   answerResp.Answer,
		})
		return s.outbound.SendToChat(ctx, *msg.ClientID, answerResp.Answer)
	}

	return s.sendFullNote(ctx, msg, "MODE_NOT_ALLOWED", factsResp, answerResp.Answer, answerResp.Mode)
}

// ------------------------------------------------------------

func (s *service) selectFacts(
	ctx context.Context,
	history []ai.Message,
	lastUserText string,
	clientInfo string,
	integrationData string,
) (aiFacts, error) {

	input := map[string]any{
		"history":                 history,
		"last_user_text":          lastUserText,
		"client_info":             clientInfo,
		"client_integration_data": integrationData,
		"cases":                   NotVPNDomainPrompt,
	}

	b, _ := json.Marshal(input)

	raw, err := s.ai.GetReply(ctx, FactSelectorPrompt, string(b))
	if err != nil {
		return aiFacts{}, err
	}

	var resp aiFacts
	_ = json.Unmarshal([]byte(raw), &resp)
	return resp, nil
}

func (s *service) validateFacts(
	ctx context.Context,
	history []ai.Message,
	lastUserText string,
	facts []string,
) (bool, error) {

	input := map[string]any{
		"history":        history,
		"last_user_text": lastUserText,
		"facts":          facts,
	}

	b, _ := json.Marshal(input)

	raw, err := s.ai.GetReply(ctx, FactValidatorPrompt, string(b))
	if err != nil {
		return false, err
	}

	var resp struct {
		Mode string `json:"mode"`
	}
	_ = json.Unmarshal([]byte(raw), &resp)

	return resp.Mode == "SELF_CONFIDENCE", nil
}

func (s *service) buildAnswer(
	ctx context.Context,
	history []ai.Message,
	lastUserText string,
	facts []string,
) (aiAnswer, error) {

	input := map[string]any{
		"history":        history,
		"last_user_text": lastUserText,
		"facts":          facts,
	}

	b, _ := json.Marshal(input)

	raw, err := s.ai.GetReply(ctx, AnswerBuilderPrompt, string(b))
	if err != nil {
		return aiAnswer{}, err
	}

	var resp aiAnswer
	_ = json.Unmarshal([]byte(raw), &resp)
	return resp, nil
}

func (s *service) validateAnswer(
	ctx context.Context,
	lastUserText string,
	answer string,
	facts []string,
) (bool, error) {

	input := map[string]any{
		"last_user_text": lastUserText,
		"answer":         answer,
		"facts":          facts,
	}

	b, _ := json.Marshal(input)

	raw, err := s.ai.GetReply(ctx, AnswerValidatorPrompt, string(b))
	if err != nil {
		return false, err
	}

	var resp struct {
		Mode string `json:"mode"`
	}
	_ = json.Unmarshal([]byte(raw), &resp)

	return resp.Mode == "SELF_CONFIDENCE", nil
}

// ------------------------------------------------------------

func (s *service) sendNote(ctx context.Context, msg *Message, reason string) error {
	note := "[AI PIPELINE]\n" + reason
	log.Println("[AI][NOTE]", reason)
	return s.outbound.SendNote(ctx, *msg.ClientID, note)
}

func (s *service) sendFullNote(
	ctx context.Context,
	msg *Message,
	stage string,
	facts aiFacts,
	answer string,
	mode string,
) error {

	note := `
[AI PIPELINE FAIL]

Stage: ` + stage + `
Mode: ` + mode + `

Facts:
` + strings.Join(facts.Facts, "\n") + `

Answer:
` + answer

	return s.outbound.SendNote(ctx, *msg.ClientID, note)
}

func (s *service) SaveOnly(ctx context.Context, msg *Message) error {
	log.Printf("[svc] save only chatId=%s sender=%s text=%q",
		msg.ChatID, msg.Sender, msg.Text,
	)
	return s.repo.SaveMessage(ctx, msg)
}

func short(s string) string {
	if len(s) > 180 {
		return s[:180] + "..."
	}
	return s
}

func (s *service) logStage(stage string, payload any) {
	b, _ := json.Marshal(payload)
	log.Printf("[AI][%s] %s", stage, short(string(b)))
}
