package pg

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vellus-ai/argoclaw/internal/providers"
)

func (s *PGSessionStore) TruncateHistory(key string, keepLast int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if data, ok := s.cache[key]; ok {
		if keepLast <= 0 {
			data.Messages = []providers.Message{}
		} else if len(data.Messages) > keepLast {
			data.Messages = data.Messages[len(data.Messages)-keepLast:]
		}
		data.Updated = time.Now()
	}
}

func (s *PGSessionStore) SetHistory(key string, msgs []providers.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if data, ok := s.cache[key]; ok {
		data.Messages = msgs
		data.Updated = time.Now()
	}
}

func (s *PGSessionStore) Reset(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if data, ok := s.cache[key]; ok {
		data.Messages = []providers.Message{}
		data.Summary = ""
		data.Updated = time.Now()
	}
}

func (s *PGSessionStore) Delete(ctx context.Context, key string) error {
	// DB DELETE first — verify tenant owns session before any side effects.
	// Evicting cache and cleaning up media files before the DB check would leave
	// inconsistent state if the session belongs to a different tenant.
	tid, err := requireTenantID(ctx)
	if err != nil {
		return err
	}
	q := "DELETE FROM sessions WHERE session_key = $1"
	args := []any{key}
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
			return fmt.Errorf("session not found or tenant mismatch: %s", key)
		}
	}

	// DB DELETE confirmed — now evict cache and clean up associated media files.
	s.mu.Lock()
	delete(s.cache, key)
	s.mu.Unlock()

	if s.OnDelete != nil {
		s.OnDelete(key)
	}
	return nil
}
