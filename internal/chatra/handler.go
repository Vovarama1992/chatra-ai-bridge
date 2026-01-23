package chatra

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	log.Println("[chatra] webhook hit")

	log.Println("[chatra HEADERS]")
	for k, v := range r.Header {
		log.Printf("%s: %v\n", k, v)
	}

	body, _ := io.ReadAll(r.Body)
	log.Printf("[chatra RAW BODY]\n%s\n", body)

	// вернуть body обратно для Decode
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	var payload struct {
		EventName string `json:"eventName"`
		Messages  []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"messages"`
		Client struct {
			ChatID string `json:"chatId"`
			ID     string `json:"id"`
		} `json:"client"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Println("[chatra] decode error:", err)
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	log.Printf(
		"[chatra] event=%s chatId=%s clientId=%s messages=%d",
		payload.EventName,
		payload.Client.ChatID,
		payload.Client.ID,
		len(payload.Messages),
	)

	// ACK СРАЗУ
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))

	if payload.EventName != "chatFragment" {
		log.Println("[chatra] skip non chatFragment")
		return
	}

	p := payload

	// ВСЯ ОБРАБОТКА — В ФОНЕ
	go func() {
		ctx := context.Background()

		for i, m := range p.Messages {
			log.Printf("[chatra] msg[%d] type=%s text=%q", i, m.Type, m.Text)

			if m.Type != "client" || m.Text == "" {
				continue
			}

			msg := &Message{
				ChatID:   p.Client.ChatID,
				Sender:   SenderClient,
				Text:     m.Text,
				ClientID: &p.Client.ID,
			}

			log.Println("[chatra] -> HandleIncoming start")

			if err := h.svc.HandleIncoming(ctx, msg); err != nil {
				log.Println("[chatra] HandleIncoming error:", err)
				continue
			}

			log.Println("[chatra] -> HandleIncoming done")
		}
	}()

	log.Println("[chatra] webhook ACK sent")
}
