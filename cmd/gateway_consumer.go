package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/vellus-ai/argoclaw/internal/agent"
	"github.com/vellus-ai/argoclaw/internal/bus"
	"github.com/vellus-ai/argoclaw/internal/channels"
	"github.com/vellus-ai/argoclaw/internal/config"
	"github.com/vellus-ai/argoclaw/internal/providers"
	"github.com/vellus-ai/argoclaw/internal/scheduler"
	"github.com/vellus-ai/argoclaw/internal/sessions"
	"github.com/vellus-ai/argoclaw/internal/store"
	"github.com/vellus-ai/argoclaw/internal/tools"
	"github.com/vellus-ai/argoclaw/pkg/protocol"
)

// consumeInboundMessages reads inbound messages from channels (Telegram, Discord, etc.)
// and routes them through the scheduler/agent loop, then publishes the response back.
// Also handles subagent announcements: routes them through the parent agent's session
// (matching TS subagent-announce.ts pattern) so the agent can reformulate for the user.
func consumeInboundMessages(ctx context.Context, msgBus *bus.MessageBus, agents *agent.Router, cfg *config.Config, sched *scheduler.Scheduler, channelMgr *channels.Manager, teamStore store.TeamStore, quotaChecker *channels.QuotaChecker, sessStore store.SessionStore, agentStore store.AgentStore, contactCollector *store.ContactCollector, postTurn tools.PostTurnProcessor, projectStore store.ProjectStore) {
	slog.Info("inbound message consumer started")

	// Inbound message deduplication (matching TS src/infra/dedupe.ts + inbound-dedupe.ts).
	// TTL=20min, max=5000 entries — prevents webhook retries / double-taps from duplicating agent runs.
	dedupe := bus.NewDedupeCache(20*time.Minute, 5000)

	// Per-session announce serialization: prevents concurrent announce runs from
	// reading stale session history. Without this, Announce #2 can start while
	// Announce #1 is still running, read history that doesn't include Announce #1's
	// messages (written only after agent loop completes), and generate responses
	// with wrong context (e.g. "waiting for Tiểu La" when Tiểu La already finished).
	var announceMu sync.Map // sessionKey → *sync.Mutex
	getAnnounceMu := func(key string) *sync.Mutex {
		v, _ := announceMu.LoadOrStore(key, &sync.Mutex{})
		return v.(*sync.Mutex)
	}

	// Track running teammate tasks so they can be cancelled when the task is
	// cancelled/failed externally (e.g. lead cancels via team_tasks tool).
	var taskRunSessions sync.Map // taskID (string) → sessionKey (string)
	msgBus.Subscribe("consumer.team-task-cancel", func(event bus.Event) {
		if event.Name != protocol.EventTeamTaskCancelled && event.Name != protocol.EventTeamTaskFailed {
			return
		}
		payload, ok := event.Payload.(protocol.TeamTaskEventPayload)
		if !ok {
			return
		}
		if sessKey, ok := taskRunSessions.Load(payload.TaskID); ok {
			if cancelled := sched.CancelSession(sessKey.(string)); cancelled {
				slog.Info("team task cancelled: stopped running agent",
					"task_id", payload.TaskID, "session", sessKey)
			}
			taskRunSessions.Delete(payload.TaskID)
		}
	})

	// Inbound debounce: merge rapid messages from the same sender before processing.
	// Matching TS createInboundDebouncer from src/auto-reply/inbound-debounce.ts.
	debounceMs := cfg.Gateway.InboundDebounceMs
	if debounceMs == 0 {
		debounceMs = 1000 // default: 1000ms
	}
	debouncer := bus.NewInboundDebouncer(
		time.Duration(debounceMs)*time.Millisecond,
		func(msg bus.InboundMessage) {
			processNormalMessage(ctx, msg, agents, cfg, sched, channelMgr, teamStore, quotaChecker, sessStore, agentStore, contactCollector, postTurn, msgBus, projectStore)
		},
	)
	defer debouncer.Stop()

	slog.Info("inbound debounce configured", "debounce_ms", debounceMs)

	for {
		msg, ok := msgBus.ConsumeInbound(ctx)
		if !ok {
			slog.Info("inbound message consumer stopped")
			return
		}

		// --- Dedup: skip duplicate inbound messages (matching TS shouldSkipDuplicateInbound) ---
		if msgID := msg.Metadata["message_id"]; msgID != "" {
			dedupeKey := fmt.Sprintf("%s|%s|%s|%s", msg.Channel, msg.SenderID, msg.ChatID, msgID)
			if dedupe.IsDuplicate(dedupeKey) {
				slog.Debug("dedup: skipping duplicate message", "key", dedupeKey)
				continue
			}
		}

		if handleSubagentAnnounce(ctx, msg, cfg, sched, channelMgr, msgBus, getAnnounceMu) {
			continue
		}
		if handleTeammateMessage(ctx, msg, cfg, sched, channelMgr, teamStore, agentStore, msgBus, postTurn, &taskRunSessions) {
			continue
		}
		// --- Handoff announce: route initial message to target agent session ---
		// Same pattern as teammate message routing, using "delegate" lane.
		if msg.Channel == tools.ChannelSystem && strings.HasPrefix(msg.SenderID, "handoff:") {
			origChannel := msg.Metadata["origin_channel"]
			origPeerKind := msg.Metadata["origin_peer_kind"]
			origLocalKey := msg.Metadata["origin_local_key"]
			origChannelType := resolveChannelType(channelMgr, origChannel)
			targetAgent := msg.AgentID
			if targetAgent == "" {
				targetAgent = cfg.ResolveDefaultAgentID()
			}
			if origPeerKind == "" {
				origPeerKind = string(sessions.PeerDirect)
			}

			if origChannel == "" || msg.ChatID == "" {
				slog.Warn("handoff announce: missing origin", "sender", msg.SenderID)
				continue
			}

			sessionKey := sessions.BuildScopedSessionKey(targetAgent, origChannel, sessions.PeerKind(origPeerKind), msg.ChatID, cfg.Sessions.Scope, cfg.Sessions.DmScope, cfg.Sessions.MainKey)
			sessionKey = overrideSessionKeyFromLocalKey(sessionKey, origLocalKey, targetAgent, origChannel, msg.ChatID, origPeerKind)

			slog.Info("handoff announce → scheduler (delegate lane)",
				"handoff", msg.SenderID,
				"to", targetAgent,
				"session", sessionKey,
			)

			announceUserID := msg.UserID
			if origPeerKind == string(sessions.PeerGroup) && msg.ChatID != "" {
				announceUserID = fmt.Sprintf("group:%s:%s", origChannel, msg.ChatID)
			}

			outMeta := buildAnnounceOutMeta(origLocalKey)

			outCh := sched.Schedule(ctx, scheduler.LaneTeam, agent.RunRequest{
				SessionKey:  sessionKey,
				Message:     msg.Content,
				Channel:     origChannel,
				ChannelType: origChannelType,
				ChatID:      msg.ChatID,
				PeerKind:    origPeerKind,
				LocalKey:    origLocalKey,
				UserID:      announceUserID,
				RunID:       fmt.Sprintf("handoff-%s", msg.Metadata["handoff_id"]),
				Stream:      false,
			})

			go func(origCh, chatID string, meta map[string]string) {
				outcome := <-outCh
				if outcome.Err != nil {
					slog.Error("handoff announce: agent run failed", "error", outcome.Err)
					return
				}
				if (outcome.Result.Content == "" && len(outcome.Result.Media) == 0) || agent.IsSilentReply(outcome.Result.Content) {
					return
				}
				outMsg := bus.OutboundMessage{
					Channel:  origCh,
					ChatID:   chatID,
					Content:  outcome.Result.Content,
					Metadata: meta,
				}
				appendMediaToOutbound(&outMsg, outcome.Result.Media)
				msgBus.PublishOutbound(outMsg)
			}(origChannel, msg.ChatID, outMeta)
			continue
		}

		// --- Teammate message: bypass debounce, route to target agent session ---
		// Same pattern as delegate announce, using "delegate" lane.
		if msg.Channel == tools.ChannelSystem && strings.HasPrefix(msg.SenderID, "teammate:") {
			origChannel := msg.Metadata["origin_channel"]
			origPeerKind := msg.Metadata["origin_peer_kind"]
			origLocalKey := msg.Metadata["origin_local_key"]
			origChannelType := resolveChannelType(channelMgr, origChannel)
			targetAgent := msg.AgentID // team_message sets AgentID to the target agent key
			if targetAgent == "" {
				targetAgent = cfg.ResolveDefaultAgentID()
			}
			if origPeerKind == "" {
				origPeerKind = string(sessions.PeerDirect)
			}

			if origChannel == "" || msg.ChatID == "" {
				slog.Warn("teammate message: missing origin — DROPPED",
					"sender", msg.SenderID,
					"target", targetAgent,
					"origin_channel", origChannel,
					"chat_id", msg.ChatID,
					"user_id", msg.UserID,
				)
				continue
			}

			sessionKey := sessions.BuildScopedSessionKey(targetAgent, origChannel, sessions.PeerKind(origPeerKind), msg.ChatID, cfg.Sessions.Scope, cfg.Sessions.DmScope, cfg.Sessions.MainKey)
			sessionKey = overrideSessionKeyFromLocalKey(sessionKey, origLocalKey, targetAgent, origChannel, msg.ChatID, origPeerKind)

			slog.Info("teammate message → scheduler (delegate lane)",
				"from", msg.SenderID,
				"to", targetAgent,
				"session", sessionKey,
			)

			announceUserID := msg.UserID
			if origPeerKind == string(sessions.PeerGroup) && msg.ChatID != "" {
				announceUserID = fmt.Sprintf("group:%s:%s", origChannel, msg.ChatID)
			}

			outMeta := buildAnnounceOutMeta(origLocalKey)

			outCh := sched.Schedule(ctx, scheduler.LaneTeam, agent.RunRequest{
				SessionKey:  sessionKey,
				Message:     msg.Content,
				Channel:     origChannel,
				ChannelType: origChannelType,
				ChatID:      msg.ChatID,
				PeerKind:    origPeerKind,
				LocalKey:    origLocalKey,
				UserID:      announceUserID,
				RunID:       fmt.Sprintf("teammate-%s-%s", msg.Metadata["from_agent"], msg.Metadata["to_agent"]),
				Stream:      false,
			})

			go func(origCh, chatID, senderID string, meta, inMeta map[string]string) {
				outcome := <-outCh

				// Auto-complete/fail the associated team task (v2 only).
				if taskIDStr := inMeta["team_task_id"]; taskIDStr != "" {
					teamTaskID, _ := uuid.Parse(taskIDStr)
					teamID, _ := uuid.Parse(inMeta["team_id"])
					if teamTaskID != uuid.Nil && teamStore != nil {
						// Only auto-complete/fail for v2 teams.
						team, _ := teamStore.GetTeam(ctx, teamID)
						if team != nil && isConsumerTeamV2(team) {
							if outcome.Err != nil {
								_ = teamStore.FailTask(ctx, teamTaskID, teamID, outcome.Err.Error())
								_ = teamStore.RecordTaskEvent(ctx, &store.TeamTaskEventData{
									TaskID:    teamTaskID,
									EventType: "failed",
									ActorType: "agent",
									ActorID:   inMeta["to_agent"],
								})
							} else {
								result := outcome.Result.Content
								if len(outcome.Result.Deliverables) > 0 {
									result = strings.Join(outcome.Result.Deliverables, "\n\n---\n\n")
								}
								if len(result) > 100_000 {
									result = result[:100_000] + "\n[truncated]"
								}
								_ = teamStore.CompleteTask(ctx, teamTaskID, teamID, result)
								_ = teamStore.RecordTaskEvent(ctx, &store.TeamTaskEventData{
									TaskID:    teamTaskID,
									EventType: "completed",
									ActorType: "agent",
									ActorID:   inMeta["to_agent"],
								})
							}
						}
					}
				}

				if outcome.Err != nil {
					slog.Error("teammate message: agent run failed", "error", outcome.Err)
					return
				}
				if (outcome.Result.Content == "" && len(outcome.Result.Media) == 0) || agent.IsSilentReply(outcome.Result.Content) {
					slog.Info("teammate message: suppressed silent/empty reply", "from", senderID)
					return
				}
				// Deliver response to origin channel (same as delegate/subagent announce).
				// This allows the lead to respond to users after receiving teammate updates.
				outMsg := bus.OutboundMessage{
					Channel:  origCh,
					ChatID:   chatID,
					Content:  outcome.Result.Content,
					Metadata: meta,
				}
				appendMediaToOutbound(&outMsg, outcome.Result.Media)
				msgBus.PublishOutbound(outMsg)
			}(origChannel, msg.ChatID, msg.SenderID, outMeta, msg.Metadata)
			continue
		}

		// --- Bot mention: route to mentioned bot (Telegram doesn't deliver bot→bot messages) ---
		// Same pattern as teammate, using "delegate" lane.
		// Session is keyed by target_channel (the mentioned bot's channel), not origin_channel.
		if msg.Channel == tools.ChannelSystem && strings.HasPrefix(msg.SenderID, "bot_mention:") {
			origChannel := msg.Metadata["origin_channel"]
			targetChannel := msg.Metadata["target_channel"]
			if targetChannel == "" {
				targetChannel = origChannel
			}
			origPeerKind := msg.Metadata["origin_peer_kind"]
			origLocalKey := msg.Metadata["origin_local_key"]
			targetChannelType := resolveChannelType(channelMgr, targetChannel)
			targetAgent := msg.AgentID
			if targetAgent == "" {
				targetAgent = cfg.ResolveDefaultAgentID()
			}
			if origPeerKind == "" {
				origPeerKind = string(sessions.PeerGroup)
			}

			if targetChannel == "" || msg.ChatID == "" {
				slog.Warn("bot mention: missing target_channel or chat_id — DROPPED",
					"sender", msg.SenderID,
					"target", targetAgent,
					"target_channel", targetChannel,
					"chat_id", msg.ChatID,
				)
				continue
			}

			sessionKey := sessions.BuildScopedSessionKey(targetAgent, targetChannel, sessions.PeerKind(origPeerKind), msg.ChatID, cfg.Sessions.Scope, cfg.Sessions.DmScope, cfg.Sessions.MainKey)
			sessionKey = overrideSessionKeyFromLocalKey(sessionKey, origLocalKey, targetAgent, targetChannel, msg.ChatID, origPeerKind)

			slog.Info("bot mention → scheduler (delegate lane)",
				"from", msg.SenderID,
				"to", targetAgent,
				"target_channel", targetChannel,
				"session", sessionKey,
			)

			announceUserID := msg.UserID
			if origPeerKind == string(sessions.PeerGroup) && msg.ChatID != "" {
				announceUserID = fmt.Sprintf("group:%s:%s", targetChannel, msg.ChatID)
			}

			outMeta := buildAnnounceOutMeta(origLocalKey)

			outCh := sched.Schedule(ctx, scheduler.LaneTeam, agent.RunRequest{
				SessionKey:  sessionKey,
				Message:     msg.Content,
				Channel:     targetChannel,
				ChannelType: targetChannelType,
				ChatID:      msg.ChatID,
				PeerKind:    origPeerKind,
				LocalKey:    origLocalKey,
				UserID:      announceUserID,
				RunID:       fmt.Sprintf("bot-mention-%s-%s", msg.Metadata["from_agent"], msg.Metadata["to_agent"]),
				Stream:      false,
			})

			go func(replyChannel, chatID string, meta map[string]string) {
				outcome := <-outCh
				if outcome.Err != nil {
					slog.Error("bot mention: agent run failed", "error", outcome.Err)
					return
				}
				if (outcome.Result.Content == "" && len(outcome.Result.Media) == 0) || agent.IsSilentReply(outcome.Result.Content) {
					slog.Debug("bot mention: suppressed silent/empty reply")
					return
				}
				outMsg := bus.OutboundMessage{
					Channel:  replyChannel,
					ChatID:   chatID,
					Content:  outcome.Result.Content,
					Metadata: meta,
				}
				appendMediaToOutbound(&outMsg, outcome.Result.Media)
				msgBus.PublishOutbound(outMsg)
			}(targetChannel, msg.ChatID, outMeta)
			continue
		}

		// --- Command: /reset — clear session history ---
		if msg.Metadata["command"] == "reset" {
			agentID := msg.AgentID
			if agentID == "" {
				agentID = resolveAgentRoute(cfg, msg.Channel, msg.ChatID, msg.PeerKind)
			}
			peerKind := msg.PeerKind
			if peerKind == "" {
				peerKind = string(sessions.PeerDirect)
			}
			sessionKey := sessions.BuildScopedSessionKey(agentID, msg.Channel, sessions.PeerKind(peerKind), msg.ChatID, cfg.Sessions.Scope, cfg.Sessions.DmScope, cfg.Sessions.MainKey)
			if msg.Metadata["is_forum"] == "true" && peerKind == string(sessions.PeerGroup) {
				var topicID int
				fmt.Sscanf(msg.Metadata["message_thread_id"], "%d", &topicID)
				if topicID > 0 {
					sessionKey = sessions.BuildGroupTopicSessionKey(agentID, msg.Channel, msg.ChatID, topicID)
				}
			}
			sessStore.Reset(sessionKey)
			sessStore.Save(context.Background(), sessionKey)
			providers.ResetCLISession("", sessionKey)
			slog.Info("inbound: /reset command", "session", sessionKey)
			continue
		}
		if handleResetCommand(msg, cfg, sessStore) {
			continue
		}
		if handleStopCommand(msg, cfg, sched, sessStore, msgBus) {
			continue
		}

		// --- Normal messages: route through debouncer ---
		debouncer.Push(msg)
	}
}

// autoSetFollowup sets followup reminders on in_progress tasks when the lead agent
// replies on a real channel. Only sets followup if the task doesn't already have one
// (respects LLM-initiated ask_user). Fire-and-forget, logs errors.
func autoSetFollowup(ctx context.Context, teamStore store.TeamStore, agentStore store.AgentStore, agentKey, channel, chatID, content string) {
	if agentStore == nil {
		return
	}
	// agentKey may be a slug ("default") or a UUID string (from WS clients).
	var ag *store.AgentData
	var err error
	if id, parseErr := uuid.Parse(agentKey); parseErr == nil {
		ag, err = agentStore.GetByID(ctx, id)
	} else {
		ag, err = agentStore.GetByKey(ctx, agentKey)
	}
	if err != nil || ag == nil {
		return
	}
	team, err := teamStore.GetTeamForAgent(ctx, ag.ID)
	if err != nil || team == nil || team.LeadAgentID != ag.ID {
		return // only lead agent triggers auto-set
	}
	// Followup is a v2 feature.
	if !isConsumerTeamV2(team) {
		return
	}

	// Skip auto-followup when lead is waiting for teammates (not user).
	if hasMember, _ := teamStore.HasActiveMemberTasks(ctx, team.ID, ag.ID); hasMember {
		slog.Debug("auto-followup: skipping, active member tasks exist", "team_id", team.ID)
		return
	}

	interval, max := parseFollowupSettings(team)
	followupAt := time.Now().Add(interval)
	msg := truncateForReminder(content, 200)

	n, err := teamStore.SetFollowupForActiveTasks(ctx, team.ID, channel, chatID, followupAt, max, msg)
	if err != nil {
		slog.Warn("auto-set followup failed", "channel", channel, "chat_id", chatID, "error", err)
	} else if n > 0 {
		slog.Info("auto-set followup: set", "channel", channel, "chat_id", chatID, "count", n, "followup_at", followupAt)
	}
}

// isConsumerTeamV2 delegates to tools.IsTeamV2 for version checking.
var isConsumerTeamV2 = tools.IsTeamV2

// parseFollowupSettings extracts followup interval and max reminders from team settings.
func parseFollowupSettings(team *store.TeamData) (time.Duration, int) {
	const (
		defaultIntervalMins = 30
		defaultMax          = 0 // unlimited
	)
	if team.Settings == nil {
		return time.Duration(defaultIntervalMins) * time.Minute, defaultMax
	}
	var settings map[string]any
	if json.Unmarshal(team.Settings, &settings) != nil {
		return time.Duration(defaultIntervalMins) * time.Minute, defaultMax
	}
	interval := defaultIntervalMins
	if v, ok := settings["followup_interval_minutes"].(float64); ok && v > 0 {
		interval = int(v)
	}
	max := defaultMax
	if v, ok := settings["followup_max_reminders"].(float64); ok && v >= 0 {
		max = int(v)
	}
	return time.Duration(interval) * time.Minute, max
}

// truncateForReminder truncates content to maxLen chars, taking the last line as context.
func truncateForReminder(content string, maxLen int) string {
	// Use last non-empty line as it's typically the most relevant.
	lines := strings.Split(strings.TrimSpace(content), "\n")
	msg := lines[len(lines)-1]
	if len(msg) > maxLen {
		msg = msg[:maxLen] + "..."
	}
	return msg
}

// appendMediaToOutbound converts agent MediaResults to outbound MediaAttachments
// on the given OutboundMessage. Handles voice annotation when applicable.
func appendMediaToOutbound(msg *bus.OutboundMessage, media []agent.MediaResult) {
	for _, mr := range media {
		msg.Media = append(msg.Media, bus.MediaAttachment{
			URL:         mr.Path,
			ContentType: mr.ContentType,
		})
		if mr.AsVoice {
			if msg.Metadata == nil {
				msg.Metadata = make(map[string]string)
			}
			msg.Metadata["audio_as_voice"] = "true"
		}
	}
}
