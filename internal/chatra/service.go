package chatra

import (
	"context"
	"encoding/json"
	"log"

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

	if err := s.repo.SaveMessage(ctx, msg); err != nil {
		return err
	}

	history, err := s.repo.GetHistory(ctx, msg.ChatID)
	if err != nil {
		return err
	}

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

	// ---------- STEP 1: CLIENT INFO ----------
	respCI, _ := s.checkClientInfo(
		ctx,
		aiHistory,
		msg.Text,
		string(clientInfo),
		string(integrationData),
	)

	mode := respCI.Mode
	answer := respCI.Answer
	reason := respCI.Reason

	if mode == "CLIENT_ONLY" {
		vRes, _ := s.validateClientOnly(
			ctx,
			aiHistory,
			msg.Text,
			answer,
			reason,
			string(clientInfo),
			string(integrationData),
		)
		mode = vRes.Mode
	}

	// ---------- STEP 2: CASES ----------
	if mode == "CASES_NEEDED" {
		raw, err := s.ai.GetReply(
			ctx,
			BaseSystemPrompt,
			NotVPNDomainPrompt,
			string(clientInfo),
			string(integrationData),
			aiHistory,
			msg.Text,
		)
		if err != nil {
			return err
		}

		var resp aiResponse
		_ = json.Unmarshal([]byte(raw), &resp)

		mode = resp.Mode
		answer = resp.Answer
		reason = resp.Reason

		if mode == "CASES_USED" || mode == "CLIENT_ONLY" {
			vRes, _ := s.validateCases(
				ctx,
				aiHistory,
				msg.Text,
				answer,
				reason,
				string(clientInfo),
				string(integrationData),
			)
			mode = vRes.Mode
		}
	}

	// ---------- FINAL SWITCH ----------
	if allowedModes[mode] {
		_ = s.repo.SaveMessage(ctx, &Message{
			ChatID: msg.ChatID,
			Sender: SenderAI,
			Text:   answer,
		})
		return s.outbound.SendToChat(ctx, *msg.ClientID, answer)
	}

	note :=
		"[AI]\nmode: " + mode + "\nreason: " + reason + "\n\n" + answer

	return s.outbound.SendNote(ctx, *msg.ClientID, note)
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

type ValidatorResult struct {
	Mode string `json:"mode"`
}

func (s *service) validateClientOnly(
	ctx context.Context,
	history []ai.Message,
	lastUserText string,
	answer string,
	reason string,
	clientInfo string,
	integrationData string,
) (*ValidatorResult, error) {

	raw, err := s.ai.GetValidationReply(
		ctx,
		ValidatorClientOnlyPrompt,
		history,
		lastUserText,
		answer,
		reason,
		clientInfo,
		integrationData,
		"", // кейсов нет
	)
	if err != nil {
		return nil, err
	}

	var res ValidatorResult
	_ = json.Unmarshal([]byte(raw), &res)
	return &res, nil
}

func (s *service) validateCases(
	ctx context.Context,
	history []ai.Message,
	lastUserText string,
	answer string,
	reason string,
	clientInfo string,
	integrationData string,
) (*ValidatorResult, error) {

	raw, err := s.ai.GetValidationReply(
		ctx,
		ValidatorCasesPrompt,
		history,
		lastUserText,
		answer,
		reason,
		clientInfo,
		integrationData,
		NotVPNDomainPrompt, // DOMAIN CASES (полный список), валидатор сам ищет CASE_* из reason
	)
	if err != nil {
		return nil, err
	}

	var res ValidatorResult
	_ = json.Unmarshal([]byte(raw), &res)
	return &res, nil
}
