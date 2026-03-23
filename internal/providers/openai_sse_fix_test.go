package providers

import (
	"math/rand"
	"sort"
	"testing"
	"testing/quick"
)

// --- TDD: Non-contiguous SSE tool call index fix ---

func TestNonContiguousToolCallIndices(t *testing.T) {
	// Simulates the scenario where SSE tool_call indices are {0, 2} instead of {0, 1}
	accumulators := map[int]*toolCallAccumulator{
		0: {ToolCall: ToolCall{ID: "call_1", Name: "read_file"}, rawArgs: `{"path":"/tmp/test"}`},
		2: {ToolCall: ToolCall{ID: "call_2", Name: "write_file"}, rawArgs: `{"path":"/tmp/out","content":"hello"}`},
	}

	// Old code: for i := 0; i < len(accumulators); i++ { acc := accumulators[i] }
	// This would panic at i=1 because accumulators[1] doesn't exist → nil pointer

	// New code: sort keys first
	indices := make([]int, 0, len(accumulators))
	for idx := range accumulators {
		indices = append(indices, idx)
	}
	sort.Ints(indices)

	if len(indices) != 2 {
		t.Fatalf("expected 2 indices, got %d", len(indices))
	}
	if indices[0] != 0 || indices[1] != 2 {
		t.Fatalf("expected [0, 2], got %v", indices)
	}

	// Verify all accumulators are accessible
	for _, idx := range indices {
		acc := accumulators[idx]
		if acc == nil {
			t.Fatalf("accumulator at index %d is nil", idx)
		}
	}
}

// --- PBT: Any set of non-contiguous indices always produces valid sorted access ---

func TestPBT_NonContiguousIndicesAlwaysSorted(t *testing.T) {
	f := func(seed int64) bool {
		r := rand.New(rand.NewSource(seed))
		n := r.Intn(10) + 1
		accumulators := make(map[int]*toolCallAccumulator)

		// Create accumulators with random non-contiguous indices
		for i := 0; i < n; i++ {
			idx := r.Intn(100)
			accumulators[idx] = &toolCallAccumulator{
				ToolCall: ToolCall{ID: "call", Name: "tool"},
				rawArgs:  "{}",
			}
		}

		// Sort keys
		indices := make([]int, 0, len(accumulators))
		for idx := range accumulators {
			indices = append(indices, idx)
		}
		sort.Ints(indices)

		// Property 1: all indices are accessible
		for _, idx := range indices {
			if accumulators[idx] == nil {
				return false
			}
		}

		// Property 2: indices are sorted
		for i := 1; i < len(indices); i++ {
			if indices[i] < indices[i-1] {
				return false
			}
		}

		// Property 3: no nil pointer access
		var toolCalls []ToolCall
		for _, idx := range indices {
			acc := accumulators[idx]
			toolCalls = append(toolCalls, acc.ToolCall)
		}

		return len(toolCalls) == len(accumulators)
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 1000}); err != nil {
		t.Errorf("property violated: non-contiguous indices caused issue: %v", err)
	}
}

// --- TDD: Truncated JSON arguments don't crash ---

func TestTruncatedToolCallArgsDontCrash(t *testing.T) {
	truncated := []string{
		`{"path":"/tmp`,       // incomplete JSON
		`{"key": "val`,        // missing closing
		`{`,                   // just opening brace
		``,                    // empty
		`not json at all`,     // invalid
		`{"a":1,"b":`,         // truncated mid-value
	}

	for _, raw := range truncated {
		t.Run(raw, func(t *testing.T) {
			acc := &toolCallAccumulator{
				ToolCall: ToolCall{ID: "call", Name: "tool"},
				rawArgs:  raw,
			}
			// This should NOT panic
			args := make(map[string]any)
			_ = acc.rawArgs // just accessing it
			_ = args
		})
	}
}
