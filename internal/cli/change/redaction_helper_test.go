// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package change

// impactEnvelope is destination-authored test scaffolding for the moved
// redaction canary: it mirrors the helper of the same name in core's
// render_test.go (which stays in eshu) so redaction_test.go runs here
// byte-identical under the moved-package freeze.
func impactEnvelope() Envelope {
	return Envelope{
		Data: map[string]any{
			"changed_file_count": float64(2),
			"truncated":          false,
			"code_surface":       map[string]any{"symbol_count": float64(3)},
			"impact_summary":     map[string]any{"direct_count": float64(1), "transitive_count": float64(2)},
			"missing_evidence":   []any{},
			"coverage":           map[string]any{"state": "supported"},
			"recommended_next_calls": []any{
				map[string]any{"tool": "get_entity_context", "reason": "expand the direct callers"},
				"not an object",
			},
		},
		Truth: map[string]any{"freshness": map[string]any{"state": "fresh"}},
	}
}
