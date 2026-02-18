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

	// --------------------------------------------------
	// STEP 1 — FACT SELECTOR
	// --------------------------------------------------

	factsResp, _ := s.selectFacts(
		ctx,
		aiHistory,
		msg.Text,
		string(clientInfo),
		string(integrationData),
	)

	if factsResp.Mode == "" {
		factsResp.Mode = "PARSE_ERROR"
	}

	currentMode := factsResp.Mode
	var finalAnswer string

	// --------------------------------------------------
	// STEP 2 — FACT VALIDATOR
	// --------------------------------------------------

	mode2, _ := s.validateFacts(ctx, aiHistory, msg.Text, factsResp.Facts)
	if mode2 != "" {
		currentMode = mode2
	}

	// --------------------------------------------------
	// STEP 3 + 4 — только если SELF_CONFIDENCE
	// --------------------------------------------------

	if currentMode == "SELF_CONFIDENCE" {

		answerResp, _ := s.buildAnswer(
			ctx,
			aiHistory,
			msg.Text,
			factsResp.Facts,
		)

		if answerResp.Mode != "" {
			currentMode = answerResp.Mode
		}

		finalAnswer = answerResp.Answer

		mode4, _ := s.validateAnswer(ctx, msg.Text, answerResp.Answer, answerResp.Facts)
		if mode4 != "" {
			currentMode = mode4
		}
	}

	// --------------------------------------------------
	// FINAL DECISION
	// --------------------------------------------------

	if allowedModes[currentMode] {
		log.Println("========== SEND TO CHAT ==========")
		log.Printf("Mode: %s", currentMode)
		log.Printf("Answer: %s", finalAnswer)

		_ = s.repo.SaveMessage(ctx, &Message{
			ChatID: msg.ChatID,
			Sender: SenderAI,
			Text:   finalAnswer,
		})

		return s.outbound.SendToChat(ctx, *msg.ClientID, finalAnswer)
	}

	if currentMode == "SELF_CONFIDENCE" {
		return s.sendFullNote(
			ctx,
			msg,
			"FINAL",
			factsResp,
			finalAnswer,
			currentMode,
		)
	}

	return nil
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
		return aiFacts{Mode: "AI_ERROR"}, err
	}

	var resp aiFacts
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		log.Println("[FACT_SELECTOR JSON ERROR]", err)
		return aiFacts{Mode: "PARSE_ERROR"}, nil
	}

	if resp.Mode == "" {
		resp.Mode = "PARSE_ERROR"
	}

	return resp, nil
}

func (s *service) validateFacts(
	ctx context.Context,
	history []ai.Message,
	lastUserText string,
	facts []string,
) (string, error) {

	input := map[string]any{
		"history":        history,
		"last_user_text": lastUserText,
		"facts":          facts,
	}

	b, _ := json.Marshal(input)

	raw, err := s.ai.GetReply(ctx, FactValidatorPrompt, string(b))
	if err != nil {
		return "AI_ERROR", err
	}

	log.Printf("[FACT_VALIDATOR][RAW] %s", short(raw))

	var resp struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		log.Printf("[FACT_VALIDATOR][JSON_ERR] %v", err)
		return "PARSE_ERROR", nil
	}

	if resp.Mode == "" {
		resp.Mode = "PARSE_ERROR"
	}

	return resp.Mode, nil
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
		return aiAnswer{Mode: "AI_ERROR"}, err
	}

	var resp aiAnswer
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		log.Println("[ANSWER_BUILDER JSON ERROR]", err)
		return aiAnswer{Mode: "PARSE_ERROR"}, nil
	}

	if resp.Mode == "" {
		resp.Mode = "PARSE_ERROR"
	}

	return resp, nil
}

func (s *service) validateAnswer(
	ctx context.Context,
	lastUserText string,
	answer string,
	facts []string,
) (string, error) {

	input := map[string]any{
		"last_user_text": lastUserText,
		"answer":         answer,
		"facts":          facts,
	}

	b, _ := json.Marshal(input)

	raw, err := s.ai.GetReply(ctx, AnswerValidatorPrompt, string(b))
	if err != nil {
		return "AI_ERROR", err
	}

	log.Printf("[ANSWER_VALIDATOR][RAW] %s", short(raw))

	var resp struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		log.Printf("[ANSWER_VALIDATOR][JSON_ERR] %v", err)
		return "PARSE_ERROR", nil
	}

	if resp.Mode == "" {
		resp.Mode = "PARSE_ERROR"
	}

	return resp.Mode, nil
}

// ------------------------------------------------------------

func (s *service) sendFullNote(
	ctx context.Context,
	msg *Message,
	stage string,
	facts aiFacts,
	answer string,
	mode string,
) error {

	note := `
[AI PIPELINE]

Stage: ` + stage + `
Mode: ` + mode + `

User question:
` + msg.Text + `

Facts:
` + strings.Join(facts.Facts, "\n") + `

Answer:
` + answer + `
`

	log.Println("========== NOTE TO OPERATOR ==========")
	log.Println(note)

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
