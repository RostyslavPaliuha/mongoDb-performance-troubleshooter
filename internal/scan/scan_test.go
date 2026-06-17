package scan

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func TestScannerCollectsSlowReadOperationsAndDeduplicatesExplains(t *testing.T) {
	provider := &fakeProvider{
		batches: [][]Operation{
			{
				{
					ID:               "op-1",
					Namespace:        "app.users",
					Operation:        "query",
					MicrosecsRunning: 75_000,
					Command: map[string]any{
						"find":   "users",
						"filter": map[string]any{"email": "a@example.com"},
					},
				},
				{
					ID:               "op-2",
					Namespace:        "app.users",
					Operation:        "query",
					MicrosecsRunning: 20_000,
					Command: map[string]any{
						"find":   "users",
						"filter": map[string]any{"status": "active"},
					},
				},
			},
			{
				{
					ID:               "op-1",
					Namespace:        "app.users",
					Operation:        "query",
					MicrosecsRunning: 90_000,
					Command: map[string]any{
						"find":   "users",
						"filter": map[string]any{"email": "a@example.com"},
					},
				},
			},
		},
		explain: map[string]ExecutionStats{
			"op-1": {
				Stage:             "COLLSCAN",
				TotalDocsExamined: 10000,
				TotalKeysExamined: 0,
				NReturned:         1,
			},
		},
	}

	report, err := Scanner{
		Provider:     provider,
		Duration:     3 * time.Millisecond,
		QueryTime:    50 * time.Millisecond,
		PollInterval: time.Millisecond,
	}.Scan(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("expected one finding, got %d", len(report.Findings))
	}

	finding := report.Findings[0]
	if finding.Operation.ID != "op-1" {
		t.Fatalf("expected op-1, got %q", finding.Operation.ID)
	}
	if finding.Stats.Stage != "COLLSCAN" {
		t.Fatalf("expected COLLSCAN stats, got %#v", finding.Stats)
	}
	if len(provider.explained) != 1 || provider.explained[0] != "op-1" {
		t.Fatalf("expected op-1 explained once, got %#v", provider.explained)
	}
}

func TestScannerSkipsUnsafeAggregateExplain(t *testing.T) {
	provider := &fakeProvider{
		batches: [][]Operation{
			{
				{
					ID:               "op-merge",
					Namespace:        "app.orders",
					Operation:        "command",
					MicrosecsRunning: 80_000,
					Command: map[string]any{
						"aggregate": "orders",
						"pipeline": []any{
							map[string]any{"$match": map[string]any{"status": "open"}},
							map[string]any{"$merge": "order_rollups"},
						},
					},
				},
			},
		},
	}

	report, err := Scanner{
		Provider:     provider,
		Duration:     time.Millisecond,
		QueryTime:    50 * time.Millisecond,
		PollInterval: time.Millisecond,
	}.Scan(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("expected one finding, got %d", len(report.Findings))
	}
	if report.Findings[0].ExplainSkippedReason == "" {
		t.Fatal("expected unsafe aggregate to include a skipped explain reason")
	}
	if len(provider.explained) != 0 {
		t.Fatalf("expected no explain calls, got %#v", provider.explained)
	}
}

func TestScannerSkipsUnsafeAggregateExplainForBSONPipeline(t *testing.T) {
	provider := &fakeProvider{
		batches: [][]Operation{
			{
				{
					ID:               "op-out",
					Namespace:        "app.orders",
					Operation:        "command",
					MicrosecsRunning: 80_000,
					Command: map[string]any{
						"aggregate": "orders",
						"pipeline": bson.A{
							bson.M{"$match": bson.M{"status": "open"}},
							bson.M{"$out": "order_rollups"},
						},
					},
				},
			},
		},
	}

	report, err := Scanner{
		Provider:     provider,
		Duration:     time.Millisecond,
		QueryTime:    50 * time.Millisecond,
		PollInterval: time.Millisecond,
	}.Scan(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("expected one finding, got %d", len(report.Findings))
	}
	if report.Findings[0].ExplainSkippedReason == "" {
		t.Fatal("expected unsafe BSON aggregate to include a skipped explain reason")
	}
	if len(provider.explained) != 0 {
		t.Fatalf("expected no explain calls, got %#v", provider.explained)
	}
}

type fakeProvider struct {
	batches   [][]Operation
	explain   map[string]ExecutionStats
	explained []string
}

func (p *fakeProvider) CurrentOperations(context.Context) ([]Operation, error) {
	if len(p.batches) == 0 {
		return nil, nil
	}
	batch := p.batches[0]
	p.batches = p.batches[1:]
	return batch, nil
}

func (p *fakeProvider) ExplainOperation(_ context.Context, operation Operation) (ExecutionStats, error) {
	p.explained = append(p.explained, operation.ID)
	return p.explain[operation.ID], nil
}
