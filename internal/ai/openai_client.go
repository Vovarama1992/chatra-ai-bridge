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
	basePrompt string,
	domainCases string,
	clientInfo string,
	integrationData string,
	history []Message,
	lastUserMessage string,
) (string, error) {

	const jsonGuard = `
Отвечай ТОЛЬКО валидным JSON.
Никакого текста вне JSON.
Формат строго:
{"answer":"строка","mode":"строка","reason":"строка"}
Где mode обязательно одно из:
CLIENT_ONLY
CASES_USED
NEED_OPERATOR

Если нарушишь формат — ответ будет отброшен.
`

	msgs := make([]openai.ChatCompletionMessage, 0)

	// 1) SYSTEM — как работать
	msgs = append(msgs, openai.ChatCompletionMessage{
		Role:    "system",
		Content: basePrompt,
	})

	// 2) USER — база кейсов (DOMAIN CASES)
	if domainCases != "" {
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role:    "user",
			Content: domainCases,
		})
	}
	// 3) CLIENT INFO
	if clientInfo != "" {
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role:    "system",
			Content: "[CLIENT INFO]\n" + clientInfo,
		})
	}

	// 4) CLIENT INTEGRATION DATA
	if integrationData != "" {
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role:    "system",
			Content: "[CLIENT INTEGRATION DATA]\n" + integrationData,
		})
	}

	// 5) История
	for _, m := range history {
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role:    m.Role,
			Content: m.Text,
		})
	}

	// 6) Последнее сообщение — главный сигнал
	msgs = append(msgs, openai.ChatCompletionMessage{
		Role:    "user",
		Content: lastUserMessage,
	})

	// 7) JSON guard в конце
	msgs = append(msgs, openai.ChatCompletionMessage{
		Role:    "system",
		Content: jsonGuard,
	})

	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    c.model,
		Messages: msgs,
	})
	if err != nil {
		log.Println("[ai] OpenAI error:", err)
		return "", err
	}

	if len(resp.Choices) == 0 {
		log.Println("[ai] empty choices")
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
) (string, error) {

	input := struct {
		History        []Message `json:"history"`
		LastUserText   string    `json:"last_user_text"`
		ProposedAnswer string    `json:"proposed_answer"`
		Reason         string    `json:"reason"`
	}{
		History:        history,
		LastUserText:   lastUserText,
		ProposedAnswer: proposedAnswer,
		Reason:         reason,
	}

	b, _ := json.Marshal(input)

	msgs := []openai.ChatCompletionMessage{
		{
			Role:    "system",
			Content: validationPrompt,
		},
		{
			Role:    "user",
			Content: string(b),
		},
	}

	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    c.model,
		Messages: msgs,
	})
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", nil
	}

	raw := resp.Choices[0].Message.Content

	log.Println("[ai] VALIDATION JSON >>>")
	log.Println(raw)
	log.Println("<<< END VALIDATION JSON")

	return raw, nil
}
