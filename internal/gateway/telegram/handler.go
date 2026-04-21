package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/yurika0211/luckyharness/internal/agent"
	"github.com/yurika0211/luckyharness/internal/gateway"
)

// Handler processes Telegram bot commands and messages.
type Handler struct {
	adapter *Adapter
	agent   *agent.Agent
}

// NewHandler creates a new Telegram command handler.
func NewHandler(adapter *Adapter, a *agent.Agent) *Handler {
	return &Handler{
		adapter: adapter,
		agent:   a,
	}
}

// HandleMessage processes an incoming gateway message.
func (h *Handler) HandleMessage(ctx context.Context, msg *gateway.Message) error {
	if msg.IsCommand {
		return h.handleCommand(ctx, msg)
	}

	// Regular text in private chats → forward to Agent
	if msg.Chat.Type == gateway.ChatPrivate {
		return h.handleChat(ctx, msg, msg.Text)
	}

	// Group chats: only respond if mentioned or replied to (already filtered by adapter)
	return h.handleChat(ctx, msg, msg.Text)
}

// handleCommand dispatches bot commands.
func (h *Handler) handleCommand(ctx context.Context, msg *gateway.Message) error {
	switch msg.Command {
	case "start":
		return h.handleStart(ctx, msg)
	case "help":
		return h.handleHelp(ctx, msg)
	case "chat":
		return h.handleChat(ctx, msg, msg.Args)
	case "model":
		return h.handleModel(ctx, msg)
	case "soul":
		return h.handleSoul(ctx, msg)
	case "tools":
		return h.handleTools(ctx, msg)
	case "reset":
		return h.handleReset(ctx, msg)
	default:
		return h.adapter.Send(ctx, msg.Chat.ID, fmt.Sprintf("Unknown command: /%s\nType /help for available commands.", msg.Command))
	}
}

// handleStart sends a welcome message.
func (h *Handler) handleStart(ctx context.Context, msg *gateway.Message) error {
	welcome := `🍀 *LuckyHarness Bot*

I'm an AI assistant powered by LuckyHarness.

*Available commands:*
/chat _message_ — Send a message to the AI
/model — Show current model
/soul — Show current SOUL info
/tools — List available tools
/reset — Reset conversation
/help — Show this help

You can also just type a message directly!`

	return h.adapter.Send(ctx, msg.Chat.ID, welcome)
}

// handleHelp lists available commands.
func (h *Handler) handleHelp(ctx context.Context, msg *gateway.Message) error {
	help := `*Available Commands:*

/start — Welcome message
/help — This help message
/chat _message_ — Send a message to the AI
/model \[name] — Get/set current model
/soul — Show current SOUL info
/tools — List available tools
/reset — Reset conversation

*Tips:*
• In private chats, just type your message directly
• In groups, mention @bot or reply to a bot message`

	return h.adapter.Send(ctx, msg.Chat.ID, help)
}

// handleChat sends a message to the agent and returns the response.
func (h *Handler) handleChat(ctx context.Context, msg *gateway.Message, text string) error {
	if strings.TrimSpace(text) == "" {
		return h.adapter.Send(ctx, msg.Chat.ID, "Please provide a message. Usage: /chat <message>")
	}

	response, err := h.agent.Chat(ctx, text)
	if err != nil {
		errMsg := fmt.Sprintf("❌ Error: %s", err.Error())
		if len(errMsg) > 200 {
			errMsg = errMsg[:200] + "..."
		}
		return h.adapter.Send(ctx, msg.Chat.ID, errMsg)
	}

	if msg.ReplyTo != nil {
		return h.adapter.SendWithReply(ctx, msg.Chat.ID, msg.ID, response)
	}
	return h.adapter.Send(ctx, msg.Chat.ID, response)
}

// handleModel shows or sets the current model.
func (h *Handler) handleModel(ctx context.Context, msg *gateway.Message) error {
	if msg.Args == "" {
		// Show current model
		cfg := h.agent.Config().Get()
		return h.adapter.Send(ctx, msg.Chat.ID, fmt.Sprintf("Current model: %s (provider: %s)", cfg.Model, cfg.Provider))
	}

	// Set model
	if err := h.agent.SwitchModel(msg.Args); err != nil {
		return h.adapter.Send(ctx, msg.Chat.ID, fmt.Sprintf("❌ Failed to switch model: %s", err.Error()))
	}

	return h.adapter.Send(ctx, msg.Chat.ID, fmt.Sprintf("✅ Switched to model: %s", msg.Args))
}

// handleSoul shows the current SOUL info.
func (h *Handler) handleSoul(ctx context.Context, msg *gateway.Message) error {
	s := h.agent.Soul()
	if s == nil {
		return h.adapter.Send(ctx, msg.Chat.ID, "No SOUL configured.")
	}

	prompt := s.SystemPrompt()
	if len(prompt) > 500 {
		prompt = prompt[:500] + "..."
	}

	return h.adapter.Send(ctx, msg.Chat.ID, fmt.Sprintf("🧠 *Current SOUL:*\n\n%s", prompt))
}

// handleTools lists available tools.
func (h *Handler) handleTools(ctx context.Context, msg *gateway.Message) error {
	tools := h.agent.Tools()
	allTools := tools.List()

	if len(allTools) == 0 {
		return h.adapter.Send(ctx, msg.Chat.ID, "No tools available.")
	}

	var sb strings.Builder
	sb.WriteString("🔧 *Available Tools:*\n\n")

	for i, t := range allTools {
		if i >= 20 {
			sb.WriteString(fmt.Sprintf("\n... and %d more", len(allTools)-20))
			break
		}
		status := "✅"
		if !t.Enabled {
			status = "❌"
		}
		sb.WriteString(fmt.Sprintf("%s %s — %s\n", status, t.Name, t.Description))
	}

	return h.adapter.Send(ctx, msg.Chat.ID, sb.String())
}

// handleReset resets the conversation.
func (h *Handler) handleReset(ctx context.Context, msg *gateway.Message) error {
	// Create a new session by starting fresh
	return h.adapter.Send(ctx, msg.Chat.ID, "🔄 Conversation reset. Starting fresh!")
}