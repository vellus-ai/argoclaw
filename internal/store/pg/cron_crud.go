package pg

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/vellus-ai/argoclaw/internal/store"
)

func (s *PGCronStore) AddJob(ctx context.Context, name string, schedule store.CronSchedule, message string, deliver bool, channel, to, agentID, userID string) (*store.CronJob, error) {
	// Apply default timezone for cron expressions when not set per-job.
	if schedule.TZ == "" && schedule.Kind == "cron" && s.defaultTZ != "" {
		schedule.TZ = s.defaultTZ
	}
	if schedule.TZ != "" {
		if _, err := time.LoadLocation(schedule.TZ); err != nil {
			return nil, fmt.Errorf("invalid timezone: %s", schedule.TZ)
		}
	}

	payload := store.CronPayload{
		Kind: "agent_turn", Message: message, Deliver: deliver, Channel: channel, To: to,
	}
	payloadJSON, _ := json.Marshal(payload)

	id := uuid.Must(uuid.NewV7())
	now := time.Now()
	scheduleKind := schedule.Kind
	deleteAfterRun := schedule.Kind == "at"

	var cronExpr, tz *string
	var runAt *time.Time
	if schedule.Expr != "" {
		cronExpr = &schedule.Expr
	}
	if schedule.AtMS != nil {
		t := time.UnixMilli(*schedule.AtMS)
		runAt = &t
	}
	if schedule.TZ != "" {
		tz = &schedule.TZ
	}

	var agentUUID *uuid.UUID
	if agentID != "" {
		aid, err := uuid.Parse(agentID)
		if err == nil {
			agentUUID = &aid
		}
	}

	var userIDPtr *string
	if userID != "" {
		userIDPtr = &userID
	}

	var intervalMS *int64
	if schedule.EveryMS != nil {
		intervalMS = schedule.EveryMS
	}

	nextRun := computeNextRun(&schedule, now, s.defaultTZ)

	tid := tenantIDFromCtx(ctx)
	var tenantIDPtr *uuid.UUID
	if tid != uuid.Nil {
		tenantIDPtr = &tid
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO cron_jobs (id, agent_id, user_id, name, enabled, schedule_kind, cron_expression, run_at, timezone,
		 interval_ms, payload, delete_after_run, next_run_at, created_at, updated_at, tenant_id)
		 VALUES ($1, $2, $3, $4, true, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		id, agentUUID, userIDPtr, name, scheduleKind, cronExpr, runAt, tz,
		intervalMS, payloadJSON, deleteAfterRun, nextRun, now, now, tenantIDPtr,
	)
	if err != nil {
		return nil, fmt.Errorf("create cron job: %w", err)
	}

	s.cacheLoaded = false // invalidate cache

	job, _ := s.GetJob(ctx, id.String())
	return job, nil
}

func (s *PGCronStore) GetJob(ctx context.Context, jobID string) (*store.CronJob, bool) {
	id, err := uuid.Parse(jobID)
	if err != nil {
		return nil, false
	}
	tid := tenantIDFromCtx(ctx)
	job, err := s.scanJobTenant(id, tid)
	if err != nil {
		return nil, false
	}
	return job, true
}

func (s *PGCronStore) ListJobs(ctx context.Context, includeDisabled bool, agentID, userID string) []store.CronJob {
	tid := tenantIDFromCtx(ctx)
	q := `SELECT id, agent_id, user_id, name, enabled, schedule_kind, cron_expression, run_at, timezone,
		 interval_ms, payload, delete_after_run, next_run_at, last_run_at, last_status, last_error,
		 created_at, updated_at FROM cron_jobs WHERE 1=1`

	var args []any
	argIdx := 1

	if tid != uuid.Nil {
		q += fmt.Sprintf(" AND tenant_id = $%d", argIdx)
		args = append(args, tid)
		argIdx++
	}
	if !includeDisabled {
		q += fmt.Sprintf(" AND enabled = $%d", argIdx)
		args = append(args, true)
		argIdx++
	}
	if agentID != "" {
		if aid, err := uuid.Parse(agentID); err == nil {
			q += fmt.Sprintf(" AND agent_id = $%d", argIdx)
			args = append(args, aid)
			argIdx++
		}
	}
	if userID != "" {
		q += fmt.Sprintf(" AND user_id = $%d", argIdx)
		args = append(args, userID)
		argIdx++
	}

	q += " ORDER BY created_at DESC"

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var result []store.CronJob
	for rows.Next() {
		job, err := scanCronRow(rows)
		if err != nil {
			continue
		}
		result = append(result, *job)
	}
	return result
}

func (s *PGCronStore) RemoveJob(ctx context.Context, jobID string) error {
	id, err := uuid.Parse(jobID)
	if err != nil {
		return fmt.Errorf("invalid job ID: %s", jobID)
	}
	tid := tenantIDFromCtx(ctx)
	q := "DELETE FROM cron_jobs WHERE id = $1"
	args := []any{id}
	if tid != uuid.Nil {
		q += " AND tenant_id = $2"
		args = append(args, tid)
	}
	result, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}
	if tid != uuid.Nil {
		if n, _ := result.RowsAffected(); n == 0 {
			return fmt.Errorf("job not found or tenant mismatch: %s", jobID)
		}
	}
	s.cacheLoaded = false
	return nil
}

func (s *PGCronStore) EnableJob(ctx context.Context, jobID string, enabled bool) error {
	id, err := uuid.Parse(jobID)
	if err != nil {
		return fmt.Errorf("invalid job ID: %s", jobID)
	}
	tid := tenantIDFromCtx(ctx)
	q := "UPDATE cron_jobs SET enabled = $1, updated_at = $2 WHERE id = $3"
	args := []any{enabled, time.Now(), id}
	if tid != uuid.Nil {
		q += " AND tenant_id = $4"
		args = append(args, tid)
	}
	_, err = s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}
	s.cacheLoaded = false
	return nil
}
