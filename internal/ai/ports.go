package ai

import "context"

// AI — внешний интеллект, не знает ни про Chatra, ни про БД
type AI interface {
	GetReply(
		ctx context.Context,
		basePrompt string,
		domainCases string,
		clientInfo string,
		integrationData string,
		history []Message,
		lastUserMessage string,
	) (string, error)
}

// Message — универсальный формат диалога для AI
type Message struct {
	Role string // "user" | "assistant" | "system"
	Text string
}
