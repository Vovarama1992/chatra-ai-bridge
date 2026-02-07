package ai

import (
	"context"
	"encoding/json"
	"log"
	"os"

	openai "github.com/sashabaranov/go-openai"
)

type OpenAIClient struct {
	client *openai.Client
	model  string
}

func NewOpenAIClient() *OpenAIClient {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY not set")
	}

	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = openai.GPT4oMini
	}

	return &OpenAIClient{
		client: openai.NewClient(apiKey),
		model:  model,
	}
}

func (c *OpenAIClient) GetReply(
	ctx context.Context,
	systemPrompt string,
	inputJSON string,
) (string, error) {

	msgs := []openai.ChatCompletionMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: inputJSON,
		},
	}

	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    c.model,
		Messages: msgs,
	})
	if err != nil {
		log.Println("[ai] OpenAI error:", err)
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", nil
	}

	raw := resp.Choices[0].Message.Content

	log.Println("[ai] RAW GPT RESPONSE >>>")
	log.Println(raw)
	log.Println("<<< END GPT RESPONSE")

	return raw, nil
}

func (c *OpenAIClient) GetValidationReply(
	ctx context.Context,
	validationPrompt string,
	history []Message,
	lastUserText string,
	proposedAnswer string,
	reason string,
	clientInfo string,
	integrationData string,
	domainCases string,
) (string, error) {

	input := struct {
		History         []Message `json:"history"`
		LastUserText    string    `json:"last_user_text"`
		ProposedAnswer  string    `json:"proposed_answer"`
		Reason          string    `json:"reason"`
		ClientInfo      string    `json:"client_info"`
		IntegrationData string    `json:"integration_data"`
		DomainCases     string    `json:"domain_cases"`
	}{
		History:         history,
		LastUserText:    lastUserText,
		ProposedAnswer:  proposedAnswer,
		Reason:          reason,
		ClientInfo:      clientInfo,
		IntegrationData: integrationData,
		DomainCases:     domainCases,
	}

	b, _ := json.Marshal(input)

	msgs := []openai.ChatCompletionMessage{
		{Role: "system", Content: validationPrompt},
		{Role: "user", Content: string(b)},
	}

	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    c.model,
		Messages: msgs,
	})
	if err != nil {
		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}
