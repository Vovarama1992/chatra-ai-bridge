package chatra

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"
)

type ChatraOutbound struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewChatraOutbound() *ChatraOutbound {
	base := os.Getenv("CHATRA_API_BASE_URL")
	token := os.Getenv("CHATRA_API_TOKEN")
	if base == "" || token == "" {
		panic("CHATRA_API_BASE_URL or CHATRA_API_TOKEN not set")
	}

	return &ChatraOutbound{
		baseURL: base,
		token:   token,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// Отправка сообщения клиенту (видит клиент)
func (c *ChatraOutbound) SendToChat(
	ctx context.Context,
	chatID string,
	text string,
) error {
	return c.send(ctx, "/chats/"+chatID+"/messages", map[string]any{
		"text": text,
	})
}

// Отправка internal note (видят только операторы)
func (c *ChatraOutbound) SendNote(
	ctx context.Context,
	chatID string,
	text string,
) error {
	return c.send(ctx, "/chats/"+chatID+"/notes", map[string]any{
		"text": text,
	})
}

func (c *ChatraOutbound) send(
	ctx context.Context,
	path string,
	body any,
) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+path,
		bytes.NewReader(b),
	)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return errors.New("chatra api error: " + resp.Status)
	}

	return nil
}
