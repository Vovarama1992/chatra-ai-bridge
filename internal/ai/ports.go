package ai

import "context"

// AI — внешний интеллект, не знает ни про Chatra, ни про БД
type AI interface {
	GetReply(
		ctx context.Context,
		systemPrompt string,
		inputJSON string,
	) (string, error)

	// Простой JSON-вызов для валидаторов
	GetValidationReply(
		ctx context.Context,
		validationPrompt string,
		history []Message,
		lastUserText string,
		proposedAnswer string,
		reason string,
		clientInfo string,
		integrationData string,
		domainCases string, // для client-only передаём ""
	) (string, error)
}

// Message — универсальный формат диалога для AI
type Message struct {
	Role string // "user" | "assistant" | "system"
	Text string
}
