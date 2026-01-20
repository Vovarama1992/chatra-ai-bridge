package chatra

import (
	"encoding/json"
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

	log.Printf("[chatra] event=%s chatId=%s clientId=%s messages=%d\n",
		payload.EventName,
		payload.Client.ChatID,
		payload.Client.ID,
		len(payload.Messages),
	)

	if payload.EventName != "chatFragment" {
		log.Println("[chatra] skip non chatFragment")
		w.WriteHeader(http.StatusOK)
		return
	}

	for i, m := range payload.Messages {
		log.Printf("[chatra] msg[%d] type=%s text=%q\n", i, m.Type, m.Text)

		if m.Type != "client" || m.Text == "" {
			continue
		}

		msg := &Message{
			ChatID:   payload.Client.ChatID,
			Sender:   SenderClient,
			Text:     m.Text,
			ClientID: &payload.Client.ID,
		}

		log.Println("[chatra] -> HandleIncoming start")

		if err := h.svc.HandleIncoming(r.Context(), msg); err != nil {
			log.Println("[chatra] HandleIncoming error:", err)
			http.Error(w, "processing error", http.StatusInternalServerError)
			return
		}

		log.Println("[chatra] -> HandleIncoming done")
	}

	log.Println("[chatra] webhook ACK")
	w.WriteHeader(http.StatusOK)
}
