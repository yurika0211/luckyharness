package gateway

import "context"

// Gateway is the interface that all messaging platform adapters must implement.
type Gateway interface {
	// Name returns the platform name (e.g., "telegram", "discord")
	Name() string

	// Start starts the gateway connection
	Start(ctx context.Context) error

	// Stop gracefully shuts down the gateway
	Stop() error

	// Send sends a message to a chat
	Send(ctx context.Context, chatID string, message string) error

	// SendWithReply sends a message as a reply to a specific message
	SendWithReply(ctx context.Context, chatID string, replyToMsgID string, message string) error

	// IsRunning returns whether the gateway is currently connected
	IsRunning() bool
}