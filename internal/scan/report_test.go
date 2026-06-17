package scan

import (
	"strings"
	"testing"
	"time"
)

func TestRenderHTMLReportIncludesDiagnosticsAndEscapesQuery(t *testing.T) {
	report := Report{
		StartedAt: time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, 6, 15, 10, 1, 0, 0, time.UTC),
		Duration:  time.Minute,
		QueryTime: 50 * time.Millisecond,
		Findings: []Finding{
			{
				Operation: Operation{
					ID:               "op-1",
					Namespace:        "app.users",
					Operation:        "query",
					MicrosecsRunning: 75_000,
					Command: map[string]any{
						"find":   "users",
						"filter": map[string]any{"name": "<script>alert(1)</script>"},
					},
				},
				Stats: ExecutionStats{
					Stage:             "COLLSCAN",
					TotalDocsExamined: 10000,
					TotalKeysExamined: 0,
					NReturned:         1,
				},
				Diagnostics: []Diagnostic{
					{
						Reason: "Collection scan examined every document.",
						Fix:    "Create an index that supports the filter and sort fields.",
					},
				},
			},
		},
	}

	html, err := RenderHTML(report)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	for _, want := range []string{
		"MongoDB Performance Troubleshooter Report",
		"app.users",
		"COLLSCAN",
		"Collection scan examined every document.",
		"Create an index that supports the filter and sort fields.",
		"&lt;script&gt;alert(1)&lt;/script&gt;",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected report to contain %q, got:\n%s", want, html)
		}
	}
	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Fatalf("expected query content to be escaped, got:\n%s", html)
	}
}
