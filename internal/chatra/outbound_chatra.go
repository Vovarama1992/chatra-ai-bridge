package chatra

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type ChatraOutbound struct {
	baseURL string
	token   string // PUBLIC:PRIVATE
	client  *http.Client
}

func NewChatraOutbound() *ChatraOutbound {
	token := strings.TrimSpace(os.Getenv("CHATRA_API_TOKEN"))
	if token == "" {
		panic("CHATRA_API_TOKEN not set (expected PUBLIC:PRIVATE)")
	}

	return &ChatraOutbound{
		baseURL: "https://app.chatra.io/api",
		token:   token,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// Отправка сообщения клиенту (рекомендовано для автоматизации)
func (c *ChatraOutbound) SendToChat(ctx context.Context, clientID string, text string) error {
	return c.send(ctx, "/pushedMessages", map[string]any{
		"clientId": clientID,
		"text":     text,
	})
}

// Internal note в REST API Chatra нет (по крайней мере в этой доке).
func (c *ChatraOutbound) SendNote(ctx context.Context, _ string, _ string) error {
	return errors.New("chatra: notes endpoint is not supported by REST API; use pushedMessages/messages")
}

func (c *ChatraOutbound) send(ctx context.Context, path string, body any) error {
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

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Chatra.Simple "+c.token)

	log.Println("[chatra] AUTH =", req.Header.Get("Authorization"))

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return errors.New(
			"chatra api error: " +
				resp.Status +
				" body=" + string(respBody),
		)
	}

	return nil
}
