# Telegram Gateway Multimedia Support Audit

**Date:** 2026-04-21  
**Scope:** LuckyHarness Telegram adapter → gateway → agent pipeline for multimedia gaps  
**Status:** 🔴 Critical — zero multimedia support end-to-end

---

## Table of Contents

1. [File-by-File Analysis](#1-file-by-file-analysis)
2. [Message Flow: Telegram → Gateway → Agent → Response](#2-message-flow-telegram--gateway--agent--response)
3. [Gap Analysis](#3-gap-analysis)
4. [Required Changes](#4-required-changes)
5. [Implementation Roadmap](#5-implementation-roadmap)

---

## 1. File-by-File Analysis

### 1.1 `internal/gateway/types.go`

**Types Defined:**

| Type | Kind | Purpose |
|------|------|---------|
| `ChatType` | `int` (iota) | Enum: `ChatPrivate`, `ChatGroup`, `ChatSuperGroup`, `ChatChannel` |
| `Chat` | struct | Chat metadata: `ID`, `Type`, `Title`, `Username` |
| `User` | struct | User identity: `ID`, `Username`, `FirstName`, `LastName` |
| `Message` | struct | Incoming message (see field breakdown below) |
| `MessageHandler` | `func` type | `func(ctx context.Context, msg *Message) error` |

**`gateway.Message` Field Breakdown:**

```go
type Message struct {
    ID        string      // Message ID
    Chat      Chat        // Chat metadata
    Sender    User        // Sender metadata
    Text      string      // ⚠️ ONLY text content — no media fields
    ReplyTo   *Message    // Reply chain
    Timestamp time.Time   // When received
    IsCommand bool        // Is this a /command?
    Command   string      // e.g., "/start"
    Args      string      // Everything after the command
}
```

**🔴 Critical Gap:** `Message` has **zero** fields for attachments, media, images, audio, video, documents, or any binary content. It is purely a text-only message type.

---

### 1.2 `internal/gateway/gateway.go`

**Types Defined:**

| Type | Kind | Purpose |
|------|------|---------|
| `Gateway` | interface | Platform adapter contract |

**`Gateway` Interface Methods:**

| Method | Signature | Notes |
|--------|-----------|-------|
| `Name()` | `() string` | Platform identifier |
| `Start()` | `(ctx context.Context) error` | Connect & begin polling |
| `Stop()` | `() error` | Graceful shutdown |
| `Send()` | `(ctx context.Context, chatID string, message string) error` | ⚠️ Text-only send |
| `SendWithReply()` | `(ctx context.Context, chatID string, replyToMsgID string, message string) error` | ⚠️ Text-only reply |
| `IsRunning()` | `() bool` | Connection status |

**🔴 Critical Gap:** The `Gateway` interface only supports sending **plain text strings**. There is no method for sending images, audio, video, documents, or any binary content. No `SendMedia()`, `SendPhoto()`, `SendDocument()`, etc.

---

### 1.3 `internal/gateway/manager.go`

**Types Defined:**

| Type | Kind | Purpose |
|------|------|---------|
| `GatewayStats` | struct | Counters: `MessagesSent`, `MessagesReceived`, `Errors` |
| `GatewayManager` | struct | Multi-gateway orchestrator |
| `GatewayStatus` | struct | JSON-serializable status per gateway |

**Key Methods:**

| Method | Purpose |
|--------|---------|
| `Register(gw Gateway)` | Add a gateway adapter |
| `Unregister(name)` | Remove and stop a gateway |
| `OnMessage(handler)` | Set the global `MessageHandler` |
| `handleMessage(ctx, gwName, msg)` | Internal dispatch — increments stats, calls handler |
| `StartAll(ctx)` / `StopAll()` | Lifecycle management |
| `Stats(name)` / `AllStats()` | Runtime statistics |
| `Status()` | JSON-serializable status snapshot |

**Gap:** `GatewayStats` only tracks text message counts. No counters for media received/sent.

---

### 1.4 `internal/gateway/telegram/adapter.go`

**Types Defined:**

| Type | Kind | Purpose |
|------|------|---------|
| `Adapter` | struct | Implements `gateway.Gateway` for Telegram |
| `rateBucket` | struct | Per-chat rate limiting state |

**Key Methods:**

| Method | Signature | Behavior |
|--------|-----------|----------|
| `NewAdapter(cfg)` | `*Adapter` | Constructor with defaults |
| `Name()` | `string` | Returns `"telegram"` |
| `SetHandler(handler)` | | Sets `gateway.MessageHandler` |
| `Start(ctx)` | `error` | Creates `tgbotapi.BotAPI`, starts `poll()` goroutine |
| `Stop()` | `error` | Cancels polling context |
| `Send(ctx, chatID, message)` | `error` | Splits text, sends chunks via `NewMessage` |
| `SendWithReply(ctx, chatID, replyToMsgID, message)` | `error` | Same + reply-to on first chunk |
| `IsRunning()` | `bool` | Running flag |
| `poll(ctx)` | (private) | Long-poll loop via `GetUpdatesChan` |
| `processUpdate(ctx, update)` | (private) | Converts & dispatches |
| `convertMessage(tgMsg)` | `*gateway.Message` | ⚠️ **Text-only conversion** |
| `isMentioned(tgMsg)` | `bool` | @bot detection |
| `sendChunk(ctx, chatID, replyTo, text)` | `error` | Sends via `tgbotapi.NewMessage` |
| `splitMessage(message)` | `[]string` | 4096-char boundary splitting |
| `waitRateLimit(chatID)` | | Per-chat rate enforcement |

**🔴 Critical Gaps in `processUpdate` (line 180–217):**

```go
func (a *Adapter) processUpdate(ctx context.Context, update tgbotapi.Update) {
    tgMsg := update.Message
    if tgMsg == nil {
        return  // ⚠️ Silently drops ALL non-Message updates
    }
    // ...
    msg := a.convertMessage(tgMsg)
    // ...
}
```

- **Line 171–173:** `if update.Message == nil { continue }` — This **silently discards** every update that isn't a text-bearing `Message`. Callback queries, edited messages, inline queries, channel posts — all dropped.
- **No handling for:** `update.Message.Photo`, `update.Message.Voice`, `update.Message.Video`, `update.Message.Document`, `update.Message.Audio`, `update.Message.VideoNote`, `update.Message.Sticker`, `update.Message.Animation`, `update.Message.Contact`, `update.Message.Location`

**🔴 Critical Gaps in `convertMessage` (line 220–263):**

```go
func (a *Adapter) convertMessage(tgMsg *tgbotapi.Message) *gateway.Message {
    // ...
    msg := &gateway.Message{
        // ...
        Text: tgMsg.Text,  // ⚠️ Only captures Text, never Caption
    }
    // ...
}
```

- **Only reads `tgMsg.Text`** — never reads `tgMsg.Caption` (which is where Telegram puts text for photo/video/document messages)
- **No extraction of any media fields** from `tgbotapi.Message`:
  - `tgMsg.Photo` — array of `PhotoSize` (thumbnails + full image)
  - `tgMsg.Voice` — `Voice` struct (OGG audio)
  - `tgMsg.Video` — `Video` struct
  - `tgMsg.Document` — `Document` struct (arbitrary files)
  - `tgMsg.Audio` — `Audio` struct (MP3 etc.)
  - `tgMsg.VideoNote` — `VideoNote` struct (round video)
  - `tgMsg.Sticker` — `Sticker` struct
  - `tgMsg.Animation` — `Animation` struct (GIF)
- **No file download logic** — Telegram requires a separate `bot.GetFileDirectURL(fileID)` call to get a downloadable URL for any media
- **No media type detection** — no way to know if a message contained media

**🔴 Critical Gaps in `sendChunk` (line 290–308):**

```go
func (a *Adapter) sendChunk(_ context.Context, chatID int64, replyTo int, text string) error {
    msg := tgbotapi.NewMessage(chatID, text)  // ⚠️ Always creates a text message
    // ...
}
```

- Only ever calls `tgbotapi.NewMessage` — never `NewPhotoUpload`, `NewAudioUpload`, `NewDocumentUpload`, `NewVideoUpload`, etc.
- No method on the `Adapter` to send any media type

---

### 1.5 `internal/gateway/telegram/config.go`

**Types Defined:**

| Type | Kind | Purpose |
|------|------|---------|
| `Config` | struct | Telegram adapter configuration |

**`Config` Fields:**

| Field | Type | Default | Purpose |
|-------|------|---------|---------|
| `Token` | `string` | — | Bot token (required) |
| `AllowedChats` | `[]string` | `[]` (allow all) | Chat ID whitelist |
| `AdminIDs` | `[]string` | `[]` | Admin user IDs |
| `MaxMessageLen` | `int` | 4000 | Text split threshold |
| `RateLimit` | `int` | 1 | Msgs/sec per chat |
| `PollTimeout` | `int` | 30 | Long-poll timeout |

**Gap:** No configuration for:
- Max file download size
- Allowed media types / MIME type whitelist
- Media storage path / temp directory
- Whether to download media or just pass URLs
- Audio transcription service config

---

### 1.6 `internal/gateway/telegram/handler.go`

**Types Defined:**

| Type | Kind | Purpose |
|------|------|---------|
| `Handler` | struct | Bridges Telegram adapter ↔ Agent |

**Key Methods:**

| Method | Purpose |
|--------|---------|
| `NewHandler(adapter, agent)` | Constructor |
| `HandleMessage(ctx, msg)` | Main dispatch — commands vs chat |
| `handleChat(ctx, msg, text)` | Forwards text to `agent.ChatWithSession()` |
| `handleCommand(ctx, msg)` | Dispatches `/start`, `/help`, `/chat`, etc. |
| `handleStart/Help/Model/Soul/Tools/Reset/History/Session` | Individual command handlers |

**🔴 Critical Gap in `HandleMessage` (line 73–85):**

```go
func (h *Handler) HandleMessage(ctx context.Context, msg *gateway.Message) error {
    if msg.IsCommand {
        return h.handleCommand(ctx, msg)
    }
    // Regular text in private chats → forward to Agent
    if msg.Chat.Type == gateway.ChatPrivate {
        return h.handleChat(ctx, msg, msg.Text)  // ⚠️ Only passes msg.Text
    }
    return h.handleChat(ctx, msg, msg.Text)  // ⚠️ Only passes msg.Text
}
```

- **Only uses `msg.Text`** — no awareness of media attachments
- **No fallback for media-only messages** — if a user sends a photo with no caption, `msg.Text` is empty, and `handleChat` returns "Please provide a message"
- **No media processing pipeline** — no call to `multimodal.Processor`

---

### 1.7 `internal/multimodal/multimodal.go`

**Types Defined:**

| Type | Kind | Purpose |
|------|------|---------|
| `Modality` | `string` | Enum: `"text"`, `"image"`, `"audio"`, `"video"` |
| `Input` | struct | Multi-modal input item |
| `AnalysisResult` | struct | Result of analyzing an input |
| `Provider` | interface | Pluggable analysis backend |
| `StreamChunk` | struct | Streaming analysis chunk |
| `Processor` | struct | Provider router & orchestrator |

**`Input` Fields:**

| Field | Type | Purpose |
|-------|------|---------|
| `ID` | `string` | Unique identifier |
| `Modality` | `Modality` | `"text"` / `"image"` / `"audio"` / `"video"` |
| `MimeType` | `string` | e.g., `"image/png"`, `"audio/ogg"` |
| `Data` | `[]byte` | Raw binary content |
| `URL` | `string` | Remote URL (alternative to Data) |
| `FilePath` | `string` | Local file path (alternative to Data) |
| `Metadata` | `map[string]string` | Arbitrary key-value metadata |
| `CreatedAt` | `time.Time` | Timestamp |

**`AnalysisResult` Fields:**

| Field | Type | Purpose |
|-------|------|---------|
| `InputID` | `string` | Links back to Input |
| `Modality` | `Modality` | What was analyzed |
| `Text` | `string` | **Extracted/understood text** — the key output |
| `Summary` | `string` | Brief summary |
| `Labels` | `[]string` | Classification tags |
| `Confidence` | `float64` | 0–1 score |
| `Metadata` | `map[string]string` | Extra info |
| `Duration` | `time.Duration` | Processing time |
| `Error` | `string` | Error if any |

**`Provider` Interface:**

| Method | Signature |
|--------|-----------|
| `Name()` | `() string` |
| `SupportedModalities()` | `[]Modality` |
| `Analyze(ctx, input)` | `(*AnalysisResult, error)` |
| `AnalyzeStream(ctx, input)` | `(<-chan StreamChunk, error)` |
| `Validate()` | `error` |

**`Processor` Methods:**

| Method | Purpose |
|--------|---------|
| `RegisterProvider(provider, isDefault, modalities...)` | Register a provider for modalities |
| `Analyze(ctx, input)` | Analyze using default provider |
| `AnalyzeStream(ctx, input)` | Stream analysis |
| `AnalyzeWithProvider(ctx, name, input)` | Use specific named provider |
| `SupportedModalities()` | List registered modalities |
| `ProvidersForModality(modality)` | List providers for a modality |

**Helper Functions:**

| Function | Purpose |
|----------|---------|
| `NewInputFromReader(modality, mimeType, reader)` | Create Input from io.Reader |
| `NewInputFromURL(modality, url)` | Create Input from URL |
| `NewInputFromPath(modality, filePath)` | Create Input from file path |

**Assessment:** The multimodal module is **well-designed and ready to use** — it supports image, audio, and video analysis through pluggable providers. However, **nothing in the codebase calls it**. It is completely disconnected from the gateway/agent pipeline.

---

### 1.8 `internal/multimodal/local_provider.go`

**Types Defined:**

| Type | Kind | Purpose |
|------|------|---------|
| `LocalProvider` | struct | Local text/metadata extraction (no AI) |

**Behavior by Modality:**

| Modality | Output | Confidence |
|----------|--------|------------|
| `text` | Raw text content | 1.0 |
| `image` | `[Image: mime/type, N bytes]` — metadata only | 0.5 |
| `audio` | `[Audio: mime/type, N bytes]` — metadata only | 0.3 |
| `video` | `[Video: mime/type, N bytes]` — metadata only | 0.3 |

**Helper Functions:**

| Function | Purpose |
|----------|---------|
| `DetectModality(mimeType)` | Maps MIME type → `Modality` |
| `NewInput(modality, mimeType, data)` | Create Input with UUID |

**⚠️ Note:** `DetectModality` has a **panic bug** — if `mimeType` is shorter than 5 characters (e.g., `"ogg"`), `mimeType[:5]` will panic with an index out of range error. The `default` branch is also unreachable for short strings.

---

### 1.9 `internal/multimodal/openai_provider.go`

**Types Defined:**

| Type | Kind | Purpose |
|------|------|---------|
| `OpenAIVisionProvider` | struct | OpenAI Vision API integration |
| `OpenAIConfig` | struct | API key, base URL, model, max tokens |

**Build Tag:** `//go:build openai` — only compiled when `openai` build tag is set.

**Supported Modalities:** `image`, `text` only (no audio/video).

**Behavior:**

| Modality | Output |
|----------|--------|
| `image` (URL) | Placeholder: `[OpenAI Vision analysis of image: <url>]` |
| `image` (Data) | Placeholder with base64 data URL |
| `text` | Raw text passthrough |

**⚠️ Note:** The `Analyze` method is a **stub** — it returns a placeholder string, not an actual OpenAI API call. The comment says "In a real implementation, this would call the OpenAI API."

---

### 1.10 `internal/agent/loop.go`

**Types Defined:**

| Type | Kind | Purpose |
|------|------|---------|
| `LoopState` | `int` (iota) | `StateReason`, `StateAct`, `StateObserve`, `StateDone` |
| `LoopConfig` | struct | `MaxIterations`, `Timeout`, `AutoApprove` |
| `LoopResult` | struct | `Response`, `Iterations`, `ToolCalls`, `State`, `TokensUsed` |
| `toolCallLog` | struct | Tool execution record |
| `StreamEvent` | struct | Streaming event |
| `EventType` | `int` (iota) | Stream event types |

**Key Methods:**

| Method | Purpose |
|--------|---------|
| `RunLoop(ctx, userInput, cfg)` | Single-turn agent loop |
| `RunLoopWithSession(ctx, sess, userInput, cfg)` | Multi-turn with session |
| `RunLoopStream(ctx, userInput, cfg)` | Streaming variant |
| `buildMessages(userInput)` | Construct `[]provider.Message` from system prompt + memory + RAG + tools + skills + user input |
| `executeToolWithSession(name, args, autoApprove, sess)` | Execute a tool call |
| `fitContextWindow(messages)` | Trim to context window |
| `indexConversationTurn(userInput, response)` | RAG indexing |

**🔴 Critical Gap in `buildMessages` (line 408–471):**

```go
func (a *Agent) buildMessages(userInput string) []provider.Message {
    // ...
    messages = append(messages, provider.Message{Role: "user", Content: userInput})
    return messages
}
```

- **`userInput` is a `string`** — no support for multimodal content
- **`provider.Message.Content` is a `string`** — no support for OpenAI-style content parts (image_url, input_audio, etc.)
- The agent loop has **no concept of media** — it cannot pass images to vision models, audio to transcription, etc.

---

### 1.11 `internal/provider/provider.go`

**`provider.Message` Fields:**

```go
type Message struct {
    Role       string     `json:"role"`
    Content    string     `json:"content"`              // ⚠️ Text only
    ToolCallID string     `json:"tool_call_id,omitempty"`
    Name       string     `json:"name,omitempty"`
    ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}
```

**🔴 Critical Gap:** `Content` is a plain `string`. OpenAI's API supports `Content` as either a `string` or an array of content parts:

```json
{
  "role": "user",
  "content": [
    {"type": "text", "text": "What's in this image?"},
    {"type": "image_url", "image_url": {"url": "https://..."}}
  ]
}
```

The current `provider.Message` cannot represent this structure.

---

## 2. Message Flow: Telegram → Gateway → Agent → Response

```
┌─────────────────────────────────────────────────────────────────────┐
│  TELEGRAM API                                                       │
│  Sends Update with Message containing:                              │
│  • Text  • Photo[]  • Voice  • Video  • Document  • Audio          │
│  • Caption  • Sticker  • Animation  • VideoNote  • Location        │
└──────────────────────────┬──────────────────────────────────────────┘
                           │ HTTP long-poll
                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Adapter.poll()                                                     │
│  • Gets update from GetUpdatesChan                                  │
│  • if update.Message == nil → DISCARD (drops callbacks, etc.)       │
└──────────────────────────┬──────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Adapter.processUpdate()                                            │
│  • Checks chat whitelist                                            │
│  • Calls convertMessage()                                           │
│  • In groups: checks @mention / reply                               │
│  • Calls handler(ctx, msg)                                          │
│                                                                     │
│  🔴 LOSS: Photo, Voice, Video, Document, Audio, Sticker,           │
│     Animation, VideoNote, Caption — ALL DROPPED                     │
└──────────────────────────┬──────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Adapter.convertMessage()                                           │
│  • Creates gateway.Message with ONLY:                               │
│    - ID, Chat, Sender, Text, Timestamp                              │
│    - IsCommand, Command, Args                                       │
│    - ReplyTo (recursive)                                            │
│  • Reads tgMsg.Text only, never tgMsg.Caption                       │
│  • No media fields populated                                        │
│                                                                     │
│  🔴 RESULT: gateway.Message is always text-only                     │
└──────────────────────────┬──────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Handler.HandleMessage()                                            │
│  • If command → handleCommand()                                     │
│  • Otherwise → handleChat(ctx, msg, msg.Text)                       │
│  • If msg.Text is empty → "Please provide a message" error          │
│                                                                     │
│  🔴 LOSS: Media-only messages (photo with no caption) rejected      │
└──────────────────────────┬──────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Handler.handleChat()                                               │
│  • Gets/creates session for chat                                    │
│  • Calls agent.ChatWithSession(ctx, sessionID, text)                │
│                                                                     │
│  🔴 LOSS: Only text string passed, no media context                 │
└──────────────────────────┬──────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Agent.RunLoopWithSession()                                         │
│  • buildMessages(userInput) → []provider.Message                    │
│  • provider.Message{Role: "user", Content: textString}              │
│  • Calls LLM provider.Chat() or ChatWithOptions()                   │
│  • Tool call loop if needed                                         │
│  • Returns LoopResult{Response: textString}                         │
│                                                                     │
│  🔴 LOSS: LLM never sees media — Content is always a plain string  │
│  🔴 DISCONNECT: multimodal.Processor exists but is NEVER called     │
└──────────────────────────┬──────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Handler.handleChat() (response path)                               │
│  • adapter.Send(ctx, chatID, response)  OR                          │
│  • adapter.SendWithReply(ctx, chatID, msgID, response)              │
│                                                                     │
│  🔴 LOSS: Can only send text back — no photos, files, audio         │
└──────────────────────────┬──────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Adapter.Send() / sendChunk()                                       │
│  • tgbotapi.NewMessage(chatID, text)                                │
│  • Markdown parse mode with plain-text fallback                     │
│  • Message splitting at 4096 chars                                  │
│                                                                     │
│  🔴 RESULT: Only plain text messages ever reach the user            │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 3. Gap Analysis

### 3.1 Receiving Photos

| Layer | Current State | Gap |
|-------|--------------|-----|
| Telegram API | `update.Message.Photo` contains `[]PhotoSize` | Not read |
| Adapter | `convertMessage()` only reads `tgMsg.Text` | No photo extraction, no file URL resolution |
| gateway.Message | No `Attachments` / `Media` field | Cannot carry photo data |
| Handler | Only uses `msg.Text` | No photo processing path |
| Agent | `buildMessages()` takes `string` | Cannot include image in LLM prompt |
| Provider | `Message.Content` is `string` | Cannot send image_url content parts to OpenAI Vision |

### 3.2 Receiving Voice Messages

| Layer | Current State | Gap |
|-------|--------------|-----|
| Telegram API | `update.Message.Voice` contains `Voice` struct (OGG) | Not read |
| Adapter | No voice handling | No file download, no transcription trigger |
| gateway.Message | No audio field | Cannot carry voice data |
| Handler | No audio processing | No transcription pipeline |
| Multimodal | `ModalityAudio` exists, `LocalProvider` returns metadata only | No Whisper/transcription integration |
| Agent | No audio input path | Cannot process transcribed text |

### 3.3 Receiving Files (Documents)

| Layer | Current State | Gap |
|-------|--------------|-----|
| Telegram API | `update.Message.Document` contains `Document` struct | Not read |
| Adapter | No document handling | No file download |
| gateway.Message | No file/attachment field | Cannot carry document data |
| Handler | No file processing | No document analysis path |
| Agent | No file input path | Cannot analyze uploaded files |

### 3.4 Sending Media Back

| Layer | Current State | Gap |
|-------|--------------|-----|
| Gateway interface | `Send()` takes `string` only | No `SendMedia()` method |
| Adapter | `sendChunk()` uses `NewMessage()` only | No `NewPhotoUpload`, `NewDocumentUpload`, etc. |
| Handler | Only calls `adapter.Send()` | No media response path |
| Agent | `LoopResult.Response` is `string` | Cannot return media |

### 3.5 Caption Handling

| Layer | Current State | Gap |
|-------|--------------|-----|
| Telegram API | Media messages use `Caption` not `Text` | `convertMessage()` reads `Text` only |
| Result | Photo with caption → `Text=""`, caption lost | User sees "Please provide a message" |

### 3.6 Summary of All Dropped Update Types

| Telegram Update Type | Field | Current Handling |
|---------------------|-------|-----------------|
| Text message | `Text` | ✅ Works |
| Photo | `Photo` | ❌ Dropped |
| Voice | `Voice` | ❌ Dropped |
| Video | `Video` | ❌ Dropped |
| Video note (round) | `VideoNote` | ❌ Dropped |
| Audio | `Audio` | ❌ Dropped |
| Document | `Document` | ❌ Dropped |
| Sticker | `Sticker` | ❌ Dropped |
| Animation (GIF) | `Animation` | ❌ Dropped |
| Contact | `Contact` | ❌ Dropped |
| Location | `Location` | ❌ Dropped |
| Venue | `Venue` | ❌ Dropped |
| Poll | `Poll` | ❌ Dropped |
| Caption (on media) | `Caption` | ❌ Dropped |
| Edited message | `EditedMessage` | ❌ Dropped |
| Callback query | `CallbackQuery` | ❌ Dropped |
| Inline query | `InlineQuery` | ❌ Dropped |

---

## 4. Required Changes

### 4.1 `gateway/types.go` — Add Media Support to Message

```go
// Attachment represents a media attachment on a message.
type Attachment struct {
    ID        string            // Unique ID for this attachment
    Type      AttachmentType    // photo, voice, video, document, audio, etc.
    MimeType  string            // MIME type (e.g., "image/jpeg", "audio/ogg")
    URL       string            // Download URL (resolved from Telegram file_id)
    FilePath  string            // Local file path if downloaded
    Data      []byte            // Raw data if loaded in memory
    FileName  string            // Original filename (for documents)
    FileSize  int64             // Size in bytes
    Width     int               // Image/video width
    Height    int               // Image/video height
    Duration  int               // Audio/video duration in seconds
    Thumbnail *Attachment       // Thumbnail for video/document
    Metadata  map[string]string // Extra metadata
}

type AttachmentType string

const (
    AttachmentPhoto    AttachmentType = "photo"
    AttachmentVoice    AttachmentType = "voice"
    AttachmentVideo    AttachmentType = "video"
    AttachmentVideoNote AttachmentType = "video_note"
    AttachmentAudio    AttachmentType = "audio"
    AttachmentDocument AttachmentType = "document"
    AttachmentSticker  AttachmentType = "sticker"
    AttachmentAnimation AttachmentType = "animation"
)

// Add to Message struct:
type Message struct {
    ID          string
    Chat        Chat
    Sender      User
    Text        string        // Text or Caption
    Attachments []Attachment  // NEW: media attachments
    ReplyTo     *Message
    Timestamp   time.Time
    IsCommand   bool
    Command     string
    Args        string
}
```

### 4.2 `gateway/gateway.go` — Add Media Send Methods

```go
type Gateway interface {
    Name() string
    Start(ctx context.Context) error
    Stop() error
    Send(ctx context.Context, chatID string, message string) error
    SendWithReply(ctx context.Context, chatID string, replyToMsgID string, message string) error
    IsRunning() bool
    
    // NEW: Media sending
    SendPhoto(ctx context.Context, chatID string, photo Attachment, caption string) error
    SendDocument(ctx context.Context, chatID string, doc Attachment, caption string) error
    SendAudio(ctx context.Context, chatID string, audio Attachment, caption string) error
    SendVideo(ctx context.Context, chatID string, video Attachment, caption string) error
}
```

### 4.3 `telegram/adapter.go` — Extract Media from Updates

**Changes to `processUpdate`:**

```go
func (a *Adapter) processUpdate(ctx context.Context, update tgbotapi.Update) {
    // Handle callback queries
    if update.CallbackQuery != nil {
        a.handleCallback(ctx, update.CallbackQuery)
        return
    }
    
    tgMsg := update.Message
    if tgMsg == nil {
        // Also check EditedMessage
        tgMsg = update.EditedMessage
    }
    if tgMsg == nil {
        return
    }
    
    // ... existing chat whitelist and mention logic ...
    
    msg := a.convertMessage(tgMsg)
    
    // NEW: If message has no text but has attachments, generate a description
    if msg.Text == "" && len(msg.Attachments) > 0 {
        msg.Text = a.describeAttachments(msg.Attachments)
    }
    
    // ... dispatch to handler ...
}
```

**Changes to `convertMessage`:**

```go
func (a *Adapter) convertMessage(tgMsg *tgbotapi.Message) *gateway.Message {
    // ... existing field mapping ...
    
    // NEW: Use Caption for media messages
    text := tgMsg.Text
    if text == "" {
        text = tgMsg.Caption
    }
    msg.Text = text
    
    // NEW: Extract attachments
    msg.Attachments = a.extractAttachments(tgMsg)
    
    return msg
}

func (a *Adapter) extractAttachments(tgMsg *tgbotapi.Message) []gateway.Attachment {
    var attachments []gateway.Attachment
    
    if len(tgMsg.Photo) > 0 {
        // Use the largest photo size
        largest := tgMsg.Photo[len(tgMsg.Photo)-1]
        url := a.bot.GetFileDirectURL(largest.FileID)
        attachments = append(attachments, gateway.Attachment{
            ID:       largest.FileID,
            Type:     gateway.AttachmentPhoto,
            MimeType: "image/jpeg",
            URL:      url,
            Width:    largest.Width,
            Height:   largest.Height,
            FileSize: int64(largest.FileSize),
        })
    }
    
    if tgMsg.Voice != nil {
        url := a.bot.GetFileDirectURL(tgMsg.Voice.FileID)
        attachments = append(attachments, gateway.Attachment{
            ID:       tgMsg.Voice.FileID,
            Type:     gateway.AttachmentVoice,
            MimeType: "audio/ogg",
            URL:      url,
            Duration: tgMsg.Voice.Duration,
            FileSize: int64(tgMsg.Voice.FileSize),
        })
    }
    
    if tgMsg.Video != nil {
        url := a.bot.GetFileDirectURL(tgMsg.Video.FileID)
        attachments = append(attachments, gateway.Attachment{
            ID:       tgMsg.Video.FileID,
            Type:     gateway.AttachmentVideo,
            MimeType: tgMsg.Video.MimeType,
            URL:      url,
            Width:    tgMsg.Video.Width,
            Height:   tgMsg.Video.Height,
            Duration: tgMsg.Video.Duration,
            FileName: tgMsg.Video.FileName,
            FileSize: int64(tgMsg.Video.FileSize),
        })
    }
    
    if tgMsg.Document != nil {
        url := a.bot.GetFileDirectURL(tgMsg.Document.FileID)
        attachments = append(attachments, gateway.Attachment{
            ID:       tgMsg.Document.FileID,
            Type:     gateway.AttachmentDocument,
            MimeType: tgMsg.Document.MimeType,
            URL:      url,
            FileName: tgMsg.Document.FileName,
            FileSize: int64(tgMsg.Document.FileSize),
        })
    }
    
    if tgMsg.Audio != nil {
        url := a.bot.GetFileDirectURL(tgMsg.Audio.FileID)
        attachments = append(attachments, gateway.Attachment{
            ID:       tgMsg.Audio.FileID,
            Type:     gateway.AttachmentAudio,
            MimeType: tgMsg.Audio.MimeType,
            URL:      url,
            Duration: tgMsg.Audio.Duration,
            FileName: tgMsg.Audio.FileName,
            FileSize: int64(tgMsg.Audio.FileSize),
        })
    }
    
    if tgMsg.VideoNote != nil {
        url := a.bot.GetFileDirectURL(tgMsg.VideoNote.FileID)
        attachments = append(attachments, gateway.Attachment{
            ID:       tgMsg.VideoNote.FileID,
            Type:     gateway.AttachmentVideoNote,
            MimeType: "video/mp4",
            URL:      url,
            Width:    tgMsg.VideoNote.Length,
            Height:   tgMsg.VideoNote.Length,
            Duration: tgMsg.VideoNote.Duration,
            FileSize: int64(tgMsg.VideoNote.FileSize),
        })
    }
    
    if tgMsg.Sticker != nil {
        url := a.bot.GetFileDirectURL(tgMsg.Sticker.FileID)
        attachments = append(attachments, gateway.Attachment{
            ID:       tgMsg.Sticker.FileID,
            Type:     gateway.AttachmentSticker,
            MimeType: "image/webp",
            URL:      url,
            Width:    tgMsg.Sticker.Width,
            Height:   tgMsg.Sticker.Height,
            Metadata: map[string]string{
                "emoji":       tgMsg.Sticker.Emoji,
                "set_name":    tgMsg.Sticker.SetName,
                "is_animated": strconv.FormatBool(tgMsg.Sticker.IsAnimated),
            },
        })
    }
    
    if tgMsg.Animation != nil {
        url := a.bot.GetFileDirectURL(tgMsg.Animation.FileID)
        attachments = append(attachments, gateway.Attachment{
            ID:       tgMsg.Animation.FileID,
            Type:     gateway.AttachmentAnimation,
            MimeType: tgMsg.Animation.MimeType,
            URL:      url,
            Width:    tgMsg.Animation.Width,
            Height:   tgMsg.Animation.Height,
            Duration: tgMsg.Animation.Duration,
            FileName: tgMsg.Animation.FileName,
            FileSize: int64(tgMsg.Animation.FileSize),
        })
    }
    
    return attachments
}
```

**New send methods:**

```go
func (a *Adapter) SendPhoto(ctx context.Context, chatID string, photo gateway.Attachment, caption string) error {
    chatIDInt, _ := strconv.ParseInt(chatID, 10, 64)
    
    var msg tgbotapi.PhotoConfig
    if photo.FilePath != "" {
        msg = tgbotapi.NewPhotoUpload(chatIDInt, photo.FilePath)
    } else if photo.URL != "" {
        msg = tgbotapi.NewPhotoShare(chatIDInt, photo.URL)
    } else if len(photo.Data) > 0 {
        msg = tgbotapi.NewPhotoUpload(chatIDInt, tgbotapi.FileBytes{Name: "photo.jpg", Bytes: photo.Data})
    }
    msg.Caption = caption
    msg.ParseMode = tgbotapi.ModeMarkdown
    _, err := a.bot.Send(msg)
    return err
}

func (a *Adapter) SendDocument(ctx context.Context, chatID string, doc gateway.Attachment, caption string) error {
    chatIDInt, _ := strconv.ParseInt(chatID, 10, 64)
    
    var msg tgbotapi.DocumentConfig
    if doc.FilePath != "" {
        msg = tgbotapi.NewDocumentUpload(chatIDInt, doc.FilePath)
    } else if doc.URL != "" {
        msg = tgbotapi.NewDocumentShare(chatIDInt, doc.URL)
    } else if len(doc.Data) > 0 {
        msg = tgbotapi.NewDocumentUpload(chatIDInt, tgbotapi.FileBytes{Name: doc.FileName, Bytes: doc.Data})
    }
    msg.Caption = caption
    _, err := a.bot.Send(msg)
    return err
}

// Similar for SendAudio, SendVideo...
```

### 4.4 `provider/provider.go` — Support Multimodal Content

```go
// ContentPart represents a single part of a multimodal message.
type ContentPart struct {
    Type     string       `json:"type"`               // "text", "image_url", "input_audio"
    Text     string       `json:"text,omitempty"`      // For type="text"
    ImageURL *ImageURL    `json:"image_url,omitempty"` // For type="image_url"
    InputAudio *InputAudio `json:"input_audio,omitempty"` // For type="input_audio"
}

type ImageURL struct {
    URL    string `json:"url"`              // URL or data:image/...;base64,...
    Detail string `json:"detail,omitempty"` // "auto", "low", "high"
}

type InputAudio struct {
    Data   string `json:"data"`   // Base64-encoded audio
    Format string `json:"format"` // "wav", "mp3", etc.
}

// Update Message to support both string and []ContentPart:
type Message struct {
    Role       string        `json:"role"`
    Content    string        `json:"content,omitempty"`              // Text content (backward compat)
    ContentParts []ContentPart `json:"content_parts,omitempty"`     // NEW: Multimodal content
    ToolCallID string        `json:"tool_call_id,omitempty"`
    Name       string        `json:"name,omitempty"`
    ToolCalls  []ToolCall    `json:"tool_calls,omitempty"`
}
```

### 4.5 `agent/loop.go` — Process Media in buildMessages

```go
func (a *Agent) buildMessages(userInput string, attachments []gateway.Attachment) []provider.Message {
    // ... existing system prompt, memory, RAG, tools, skills ...
    
    // NEW: Build user message with multimodal content
    if len(attachments) > 0 {
        contentParts := []provider.ContentPart{
            {Type: "text", Text: userInput},
        }
        for _, att := range attachments {
            switch att.Type {
            case gateway.AttachmentPhoto:
                contentParts = append(contentParts, provider.ContentPart{
                    Type: "image_url",
                    ImageURL: &provider.ImageURL{URL: att.URL, Detail: "auto"},
                })
            case gateway.AttachmentVoice, gateway.AttachmentAudio:
                // Transcribe via multimodal processor, then add as text
                if a.multimodalProc != nil {
                    input := multimodal.NewInputFromURL(multimodal.ModalityAudio, att.URL)
                    result, err := a.multimodalProc.Analyze(ctx, input)
                    if err == nil && result.Text != "" {
                        contentParts = append(contentParts, provider.ContentPart{
                            Type: "text",
                            Text: fmt.Sprintf("[Transcribed audio]: %s", result.Text),
                        })
                    }
                }
            case gateway.AttachmentDocument:
                // Add document info as text context
                contentParts = append(contentParts, provider.ContentPart{
                    Type: "text",
                    Text: fmt.Sprintf("[Attached document: %s (%s, %d bytes)]", att.FileName, att.MimeType, att.FileSize),
                })
            }
        }
        messages = append(messages, provider.Message{Role: "user", ContentParts: contentParts})
    } else {
        messages = append(messages, provider.Message{Role: "user", Content: userInput})
    }
    
    return messages
}
```

### 4.6 `telegram/handler.go` — Route Media Messages

```go
func (h *Handler) HandleMessage(ctx context.Context, msg *gateway.Message) error {
    if msg.IsCommand {
        return h.handleCommand(ctx, msg)
    }
    
    // NEW: If message has attachments, process them
    if len(msg.Attachments) > 0 {
        return h.handleMediaMessage(ctx, msg)
    }
    
    // Regular text
    if msg.Chat.Type == gateway.ChatPrivate {
        return h.handleChat(ctx, msg, msg.Text)
    }
    return h.handleChat(ctx, msg, msg.Text)
}

func (h *Handler) handleMediaMessage(ctx context.Context, msg *gateway.Message) error {
    // Build a rich prompt from media + text
    var prompt strings.Builder
    if msg.Text != "" {
        prompt.WriteString(msg.Text)
        prompt.WriteString("\n\n")
    }
    
    for i, att := range msg.Attachments {
        switch att.Type {
        case gateway.AttachmentPhoto:
            if i == 0 {
                prompt.WriteString("[User sent a photo]")
            } else {
                prompt.WriteString(fmt.Sprintf("[User sent photo #%d]", i+1))
            }
        case gateway.AttachmentVoice:
            prompt.WriteString("[User sent a voice message]")
        case gateway.AttachmentVideo:
            prompt.WriteString("[User sent a video]")
        case gateway.AttachmentDocument:
            prompt.WriteString(fmt.Sprintf("[User sent a file: %s]", att.FileName))
        case gateway.AttachmentAudio:
            prompt.WriteString(fmt.Sprintf("[User sent audio: %s]", att.FileName))
        case gateway.AttachmentSticker:
            prompt.WriteString(fmt.Sprintf("[User sent a sticker: %s]", att.Metadata["emoji"]))
        }
        prompt.WriteString("\n")
    }
    
    return h.handleChat(ctx, msg, prompt.String())
}
```

### 4.7 `multimodal/local_provider.go` — Fix Panic Bug

```go
func DetectModality(mimeType string) Modality {
    if len(mimeType) < 5 {
        return ModalityText  // Safe default for short/empty MIME types
    }
    switch {
    case mimeType == "text/plain" || mimeType == "text/markdown" || mimeType == "text/html":
        return ModalityText
    case mimeType[:5] == "image":
        return ModalityImage
    case mimeType[:5] == "audio":
        return ModalityAudio
    case mimeType[:5] == "video":
        return ModalityVideo
    default:
        return ModalityText
    }
}
```

### 4.8 `telegram/config.go` — Add Media Configuration

```go
type Config struct {
    Token         string
    AllowedChats  []string
    AdminIDs      []string
    MaxMessageLen int
    RateLimit     int
    PollTimeout   int
    
    // NEW: Media configuration
    MaxFileSize      int64    // Max file size to download (default 20MB)
    AllowedMediaTypes []string // MIME type whitelist (empty = allow all)
    DownloadMedia    bool     // Whether to download media files (default: false, pass URLs only)
    MediaTempDir     string   // Temp directory for downloaded files
}
```

---

## 5. Implementation Roadmap

### Phase 1: Foundation (Minimal Viable Media)

**Goal:** Receive photos and voice messages, pass them to the agent as text descriptions.

| Step | File | Change | Effort |
|------|------|--------|--------|
| 1.1 | `gateway/types.go` | Add `Attachment`, `AttachmentType`, `Attachments []Attachment` to `Message` | Small |
| 1.2 | `telegram/adapter.go` | Fix `convertMessage` to read `Caption`, add `extractAttachments()` | Medium |
| 1.3 | `telegram/adapter.go` | Resolve file URLs via `bot.GetFileDirectURL()` | Small |
| 1.4 | `telegram/handler.go` | Add `handleMediaMessage()`, don't reject empty-text media messages | Small |
| 1.5 | `agent/loop.go` | Update `buildMessages` signature to accept attachments | Medium |
| 1.6 | `multimodal/local_provider.go` | Fix `DetectModality` panic bug | Trivial |

### Phase 2: Vision Integration

**Goal:** Pass photos to OpenAI Vision API for actual image understanding.

| Step | File | Change | Effort |
|------|------|--------|--------|
| 2.1 | `provider/provider.go` | Add `ContentPart`, `ImageURL`, `ContentParts` to `Message` | Medium |
| 2.2 | `provider/openai.go` | Serialize `ContentParts` as OpenAI content array format | Medium |
| 2.3 | `agent/loop.go` | Build `ContentParts` for image attachments | Medium |
| 2.4 | `multimodal/openai_provider.go` | Implement actual OpenAI Vision API call (replace stub) | Large |

### Phase 3: Voice/Audio Transcription

**Goal:** Transcribe voice messages via Whisper API.

| Step | File | Change | Effort |
|------|------|--------|--------|
| 3.1 | `multimodal/` | Add `WhisperProvider` implementing `Provider` for audio | Large |
| 3.2 | `agent/loop.go` | Transcribe voice/audio before building messages | Medium |
| 3.3 | `telegram/adapter.go` | Download OGG voice files for transcription | Medium |

### Phase 4: Sending Media Back

**Goal:** Agent can respond with images, files, and audio.

| Step | File | Change | Effort |
|------|------|--------|--------|
| 4.1 | `gateway/gateway.go` | Add `SendPhoto`, `SendDocument`, `SendAudio`, `SendVideo` to interface | Medium |
| 4.2 | `telegram/adapter.go` | Implement all send methods using `tgbotapi.NewPhotoUpload/Share`, etc. | Medium |
| 4.3 | `agent/loop.go` | Extend `LoopResult` to carry media responses | Medium |
| 4.4 | `telegram/handler.go` | Detect media in response and use appropriate send method | Medium |

### Phase 5: Polish & Edge Cases

| Step | File | Change | Effort |
|------|------|--------|--------|
| 5.1 | `telegram/adapter.go` | Handle `EditedMessage` updates | Small |
| 5.2 | `telegram/adapter.go` | Handle `CallbackQuery` for inline buttons | Medium |
| 5.3 | `telegram/config.go` | Add media config fields (max size, allowed types, temp dir) | Small |
| 5.4 | `gateway/manager.go` | Add media stats counters | Small |
| 5.5 | All | Comprehensive tests for media paths | Large |

---

## Appendix A: Telegram Bot API Media Fields Reference

| Field | Type | MIME | Notes |
|-------|------|------|-------|
| `Photo` | `[]PhotoSize` | `image/jpeg` | Multiple sizes, use largest |
| `Voice` | `Voice` | `audio/ogg` | OGG with Opus codec |
| `Video` | `Video` | varies | MP4 typically |
| `VideoNote` | `VideoNote` | `video/mp4` | Round video, fixed dimensions |
| `Audio` | `Audio` | varies | MP3 typically |
| `Document` | `Document` | varies | Any file type |
| `Sticker` | `Sticker` | `image/webp` | May be animated (TGS) |
| `Animation` | `Animation` | `video/mp4` or `image/gif` | GIF-like |

All require `bot.GetFileDirectURL(fileID)` to get a downloadable URL. Files >20MB require the `file_id` + manual download approach.

## Appendix B: OpenAI API Multimodal Content Format

```json
{
  "model": "gpt-4o",
  "messages": [
    {
      "role": "user",
      "content": [
        {"type": "text", "text": "What's in this image?"},
        {"type": "image_url", "image_url": {"url": "https://example.com/photo.jpg", "detail": "auto"}}
      ]
    }
  ]
}
```

```json
{
  "model": "gpt-4o-audio-preview",
  "messages": [
    {
      "role": "user",
      "content": [
        {"type": "text", "text": "Transcribe this audio."},
        {"type": "input_audio", "input_audio": {"data": "<base64>", "format": "wav"}}
      ]
    }
  ]
}
```

## Appendix C: Existing Code That Can Be Reused

| Component | Status | Reuse Potential |
|-----------|--------|----------------|
| `multimodal.Processor` | ✅ Complete | Direct use — register providers, call `Analyze()` |
| `multimodal.Input` | ✅ Complete | Create from URL/Data/Path — maps perfectly to Telegram media |
| `multimodal.NewInputFromURL()` | ✅ Complete | Ideal for Telegram file URLs |
| `multimodal.DetectModality()` | ⚠️ Bug | Fix panic, then use for MIME→Modality mapping |
| `multimodal.LocalProvider` | ✅ Complete | Fallback for when no AI provider is configured |
| `multimodal.OpenAIVisionProvider` | ⚠️ Stub | Needs real API implementation |
| `tgbotapi.GetFileDirectURL()` | ✅ Available | Use to resolve Telegram file URLs |
| `tgbotapi.NewPhotoUpload/Share()` | ✅ Available | Use for sending photos back |
| `tgbotapi.NewDocumentUpload/Share()` | ✅ Available | Use for sending documents back |