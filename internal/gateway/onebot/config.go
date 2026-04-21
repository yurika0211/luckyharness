package onebot

// Config holds the OneBot gateway configuration.
type Config struct {
	// HTTP API endpoint (e.g., "http://127.0.0.1:3000")
	APIBase string `yaml:"api_base" json:"api_base"`

	// Access token for OneBot API (optional)
	AccessToken string `yaml:"access_token" json:"access_token"`

	// WebSocket URL for receiving events (e.g., "ws://127.0.0.1:3000/event")
	WSURL string `yaml:"ws_url" json:"ws_url"`

	// HTTP webhook path for receiving events (alternative to WS)
	WebhookPath string `yaml:"webhook_path" json:"webhook_path"`

	// QQ bot ID (for message parsing)
	BotQQID string `yaml:"bot_qq_id" json:"bot_qq_id"`

	// Max message length before splitting
	MaxMessageLen int `yaml:"max_message_len" json:"max_message_len"`

	// Typing indicator: show "正在输入" when processing
	ShowTyping bool `yaml:"show_typing" json:"show_typing"`

	// Auto like: send a like to the user when they send a message
	AutoLike bool `yaml:"auto_like" json:"auto_like"`

	// Like times (how many likes to send, 1-10)
	LikeTimes int `yaml:"like_times" json:"like_times"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxMessageLen: 4000,
		ShowTyping:    true,
		AutoLike:      true,
		LikeTimes:     1,
	}
}