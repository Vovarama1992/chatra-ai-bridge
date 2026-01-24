package ai

import (
	"context"
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
	domainPrompt string,
	clientInfo string,
	integrationData string,
	history []Message,
	lastUserMessage string,
) (string, error) {

	const jsonGuard = `
Отвечай ТОЛЬКО валидным JSON.
Никакого текста вне JSON.
Формат строго:
{"answer":"строка","confidence":0.0}
Если нарушишь формат — ответ будет отброшен.
`

	msgs := make([]openai.ChatCompletionMessage, 0)

	// 1) Доменный промпт
	msgs = append(msgs, openai.ChatCompletionMessage{
		Role:    "system",
		Content: domainPrompt,
	})

	// 2) CLIENT INFO — факты устройства
	if clientInfo != "" {
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role:    "system",
			Content: "[CLIENT INFO]\n" + clientInfo,
		})
	}

	// 3) CLIENT INTEGRATION DATA — факты из Chatra/CRM
	if integrationData != "" {
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role:    "system",
			Content: "[CLIENT INTEGRATION DATA]\n" + integrationData,
		})
	}

	// 4) История диалога
	for _, m := range history {
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role:    m.Role,
			Content: m.Text,
		})
	}

	// 5) Последнее сообщение пользователя
	msgs = append(msgs, openai.ChatCompletionMessage{
		Role:    "user",
		Content: lastUserMessage,
	})

	// 6) JSON guard
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
