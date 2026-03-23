package channels

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/vellus-ai/argoclaw/internal/bus"
)

// botMentionRe matches Telegram @mentions (5-32 chars, [a-zA-Z0-9_]).
var botMentionRe = regexp.MustCompile(`@([a-zA-Z0-9_]{5,32})`)

// WebhookRoute holds a path and handler pair for mounting on the main gateway mux.
type WebhookRoute struct {
	Path    string
	Handler http.Handler
}

// dispatchOutbound consumes outbound messages from the bus and routes them
// to the appropriate channel. Internal channels are silently skipped.
func (m *Manager) dispatchOutbound(ctx context.Context) {
	slog.Info("outbound dispatcher started")

	for {
		select {
		case <-ctx.Done():
			slog.Info("outbound dispatcher stopped")
			return
		default:
			msg, ok := m.bus.SubscribeOutbound(ctx)
			if !ok {
				continue
			}

			// Skip internal channels
			if IsInternalChannel(msg.Channel) {
				continue
			}

			m.mu.RLock()
			channel, exists := m.channels[msg.Channel]
			m.mu.RUnlock()

			if !exists {
				slog.Warn("unknown channel for outbound message", "channel", msg.Channel)
				continue
			}

			// Filter out temp media files that no longer exist (already sent by another dispatch).
			if len(msg.Media) > 0 {
				tmpDir := os.TempDir()
				filtered := msg.Media[:0]
				for _, media := range msg.Media {
					if media.URL != "" && strings.HasPrefix(media.URL, tmpDir) {
						if _, err := os.Stat(media.URL); err != nil {
							slog.Debug("skipping already-delivered temp media", "path", media.URL)
							continue
						}
					}
					filtered = append(filtered, media)
				}
				msg.Media = filtered
				// If only media was in this message and all files are gone, skip entirely.
				if len(msg.Media) == 0 && msg.Content == "" {
					continue
				}
			}

			if err := channel.Send(ctx, msg); err != nil {
				slog.Error("error sending message to channel",
					"channel", msg.Channel,
					"error", err,
				)
				// Try to send a text-only error notification back to the chat.
				// Only for media failures — text-only failures likely mean the chat
				// is inaccessible (kicked, blocked, etc.) so retrying won't help.
				if len(msg.Media) > 0 {
					notifyMsg := bus.OutboundMessage{
						Channel:  msg.Channel,
						ChatID:   msg.ChatID,
						Content:  formatChannelSendError(err),
						Metadata: sendErrorMeta(msg.Metadata),
					}
					if err2 := channel.Send(ctx, notifyMsg); err2 != nil {
						slog.Warn("failed to send error notification",
							"channel", msg.Channel, "error", err2)
					}
				}
			} else {
				// Bot-to-bot mention routing: Telegram does not deliver bot messages to other bots.
				// When this bot's message mentions another bot, inject InboundMessage so the other bot wakes up.
				m.dispatchBotMentions(ctx, msg, channel)
			}

			// Clean up temp media files only. Workspace-generated files are preserved
			// so they remain accessible via workspace/web UI after delivery.
			tmpDir := os.TempDir()
			for _, media := range msg.Media {
				if media.URL != "" && strings.HasPrefix(media.URL, tmpDir) {
					if err := os.Remove(media.URL); err != nil {
						slog.Debug("failed to clean up media file", "path", media.URL, "error", err)
					}
				}
			}
		}
	}
}

// dispatchBotMentions publishes InboundMessage for each @mentioned bot so they receive the message.
// Telegram does not deliver bot→bot messages; this internal routing wakes up the mentioned bots.
func (m *Manager) dispatchBotMentions(ctx context.Context, msg bus.OutboundMessage, fromChannel Channel) {
	if msg.Content == "" {
		return
	}
	bmc, ok := fromChannel.(BotMentionChannel)
	if !ok {
		return
	}
	fromBotUsername := bmc.BotUsername()
	if fromBotUsername == "" {
		return
	}

	// Build map: lowercase bot username → (channelName, agentID)
	type targetInfo struct {
		channelName string
		agentID     string
	}
	targets := make(map[string]targetInfo)
	m.mu.RLock()
	for name, ch := range m.channels {
		if name == msg.Channel {
			continue
		}
		if other, ok := ch.(BotMentionChannel); ok {
			uname := other.BotUsername()
			if uname == "" {
				continue
			}
			key := strings.ToLower(uname)
			agentID := ""
			if ag, ok := ch.(interface{ AgentID() string }); ok {
				agentID = ag.AgentID()
			}
			targets[key] = targetInfo{channelName: name, agentID: agentID}
		}
	}
	m.mu.RUnlock()

	// Find unique @mentions in content
	matches := botMentionRe.FindAllStringSubmatch(msg.Content, -1)
	seen := make(map[string]bool)
	for _, submatch := range matches {
		if len(submatch) < 2 {
			continue
		}
		username := strings.ToLower(submatch[1])
		if seen[username] {
			continue
		}
		seen[username] = true
		if username == strings.ToLower(fromBotUsername) {
			continue
		}
		tgt, ok := targets[username]
		if !ok {
			continue
		}
		// Inject InboundMessage for the mentioned bot (same pattern as teammate).
		// target_channel: the mentioned bot's channel — session is keyed by this (not origin).
		// When user mentions techlead, session is agent+telegram_techlead+chatID.
		meta := map[string]string{
			"origin_channel":   msg.Channel,
			"target_channel":   tgt.channelName,
			"origin_peer_kind": "group",
			"from_agent":       fromBotUsername,
			"to_agent":         tgt.agentID,
		}
		if v := msg.Metadata["local_key"]; v != "" {
			meta["origin_local_key"] = v
		} else if msg.ChatID != "" {
			meta["origin_local_key"] = msg.ChatID
		}
		for _, k := range []string{"message_thread_id", "group_id"} {
			if v := msg.Metadata[k]; v != "" {
				meta[k] = v
			}
		}

		content := fmt.Sprintf("[Message from @%s]: %s", fromBotUsername, msg.Content)
		m.bus.PublishInbound(bus.InboundMessage{
			Channel:  "system",
			SenderID: fmt.Sprintf("bot_mention:%s:%s", msg.Channel, fromBotUsername),
			ChatID:   msg.ChatID,
			Content:  content,
			Media:    nil,
			UserID:   fmt.Sprintf("bot:%s:%s", msg.Channel, fromBotUsername),
			AgentID:  tgt.agentID,
			Metadata: meta,
		})
		slog.Info("bot mention routed",
			"from_channel", msg.Channel,
			"from_bot", fromBotUsername,
			"to_channel", tgt.channelName,
			"to_agent", tgt.agentID,
			"chat_id", msg.ChatID,
		)
	}
}

// WebhookHandlers returns all webhook handlers from channels that implement WebhookChannel.
// Used to mount webhook routes on the main gateway mux.
func (m *Manager) WebhookHandlers() []WebhookRoute {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var routes []WebhookRoute
	for _, ch := range m.channels {
		if wh, ok := ch.(WebhookChannel); ok {
			if path, handler := wh.WebhookHandler(); path != "" && handler != nil {
				routes = append(routes, WebhookRoute{Path: path, Handler: handler})
			}
		}
	}
	return routes
}

// SendToChannel delivers a message to a specific channel by name.
func (m *Manager) SendToChannel(ctx context.Context, channelName, chatID, content string) error {
	m.mu.RLock()
	channel, exists := m.channels[channelName]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("channel %s not found", channelName)
	}

	msg := bus.OutboundMessage{
		Channel: channelName,
		ChatID:  chatID,
		Content: content,
	}

	return channel.Send(ctx, msg)
}

// --- Send error notification helpers ---

// telegramAPIDescRe extracts the human-readable description from Telegram Bot API errors.
// Example: `telego: sendPhoto: api: 400 "Bad Request: not enough rights to send photos to the chat"`
//
//	→ "not enough rights to send photos to the chat"
var telegramAPIDescRe = regexp.MustCompile(`"Bad Request:\s*(.+?)"`)

// formatChannelSendError converts a channel.Send error into a user-friendly message.
// Never exposes raw library/HTTP details.
func formatChannelSendError(err error) string {
	raw := err.Error()
	lower := strings.ToLower(raw)

	// Telegram "Bad Request: <description>" — extract description
	if m := telegramAPIDescRe.FindStringSubmatch(raw); len(m) == 2 {
		return fmt.Sprintf("⚠️ Send failed: %s", m[1])
	}

	// Common Telegram API errors (non-Bad Request)
	switch {
	case strings.Contains(lower, "not enough rights"):
		return "⚠️ Send failed: bot doesn't have permission to send this type of message."
	case strings.Contains(lower, "chat not found"):
		return "⚠️ Send failed: chat not found."
	case strings.Contains(lower, "bot was blocked"):
		return "⚠️ Send failed: bot was blocked by the user."
	case strings.Contains(lower, "user is deactivated"):
		return "⚠️ Send failed: user account is deactivated."
	case strings.Contains(lower, "too many requests") || strings.Contains(lower, "flood"):
		return "⚠️ Send failed: rate limited by Telegram. Please try again later."
	case strings.Contains(lower, "file is too big") || strings.Contains(lower, "wrong file"):
		return "⚠️ Send failed: file is too large or invalid for Telegram."
	}

	// Generic fallback — don't expose internals
	return "⚠️ Failed to deliver message. Check bot logs for details."
}

// sendErrorMeta copies only the routing fields from outbound metadata.
// Strips reply_to_message_id, placeholder_key, audio_as_voice, etc.
// that could cause unintended side effects on the error notification.
func sendErrorMeta(orig map[string]string) map[string]string {
	if orig == nil {
		return nil
	}
	meta := make(map[string]string)
	for _, k := range []string{"local_key", "message_thread_id"} {
		if v := orig[k]; v != "" {
			meta[k] = v
		}
	}
	if len(meta) == 0 {
		return nil
	}
	return meta
}
