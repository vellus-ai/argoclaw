package pg_test

import (
	"github.com/vellus-ai/argoclaw/internal/store"
	"github.com/vellus-ai/argoclaw/internal/store/pg"
)

// Compile-time check: PGPluginStore must satisfy the PluginStore interface.
var _ store.PluginStore = (*pg.PGPluginStore)(nil)
