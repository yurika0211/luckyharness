Place Telegram meme images here to enable random meme replies.

Supported file types:

- .png
- .jpg
- .jpeg
- .webp
- .gif

Runtime behavior:

- The Telegram handler will look for this directory automatically when running from the repository root.
- It only tries to send a meme after short casual/completion-style replies.
- It skips meme sending when the reply already contains outbound media.
- Default probability is 12 percent per eligible reply.
- Default per-chat cooldown is 10 minutes.

Optional overrides:

- `LH_TG_MEME_DIR`
- `LH_TG_MEME_PROBABILITY`
- `LH_TG_MEME_COOLDOWN_SECONDS`
