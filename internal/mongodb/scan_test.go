package mongodb

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func TestClientCurrentOperationsRunsCurrentOpAgainstAdmin(t *testing.T) {
	runner := &fakeScanCommandRunner{
		currentOps: currentOpResult{
			Inprog: []currentOpDocument{
				{
					OpID:             "123",
					Ns:               "app.users",
					Op:               "query",
					MicrosecsRunning: 60_000,
					Command: map[string]any{
						"find": "users",
					},
				},
			},
		},
	}
	client := NewClient(runner)

	operations, err := client.CurrentOperations(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if runner.database != "admin" {
		t.Fatalf("expected admin database, got %q", runner.database)
	}
	if runner.commandName != "currentOp" {
		t.Fatalf("expected currentOp command, got %q", runner.commandName)
	}
	if len(operations) != 1 || operations[0].ID != "123" {
		t.Fatalf("unexpected operations: %#v", operations)
	}
}

func TestBuildRunCommandDocumentConvertsMultiKeyDocumentToOrderedBSON(t *testing.T) {
	document := buildRunCommandDocument(Command{
		Name: "currentOp",
		Document: map[string]any{
			"currentOp": true,
			"active":    true,
		},
	})

	ordered, ok := document.(bson.D)
	if !ok {
		t.Fatalf("expected ordered BSON document, got %T", document)
	}
	if len(ordered) != 2 {
		t.Fatalf("expected 2 command fields, got %#v", ordered)
	}
	if ordered[0].Key != "currentOp" || ordered[0].Value != true {
		t.Fatalf("expected command name first, got %#v", ordered)
	}
	if ordered[1].Key != "active" || ordered[1].Value != true {
		t.Fatalf("expected active option second, got %#v", ordered)
	}
}

func TestClientExplainOperationRunsExecutionStatsExplain(t *testing.T) {
	runner := &fakeScanCommandRunner{
		explain: explainResult{
			ExecutionStats: executionStatsDocument{
				NReturned:         2,
				TotalDocsExamined: 200,
				TotalKeysExamined: 0,
				ExecutionStages: executionStageDocument{
					Stage: "COLLSCAN",
				},
			},
		},
	}
	client := NewClient(runner)

	stats, err := client.ExplainOperation(context.Background(), ScanOperation{
		ID:               "op-1",
		Namespace:        "app.users",
		Operation:        "query",
		MicrosecsRunning: int64((60 * time.Millisecond) / time.Microsecond),
		Command: map[string]any{
			"find":   "users",
			"filter": map[string]any{"email": "a@example.com"},
		},
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if runner.database != "app" {
		t.Fatalf("expected app database, got %q", runner.database)
	}
	if runner.verbosity != "executionStats" {
		t.Fatalf("expected executionStats verbosity, got %q", runner.verbosity)
	}
	if stats.Stage != "COLLSCAN" {
		t.Fatalf("expected COLLSCAN, got %#v", stats)
	}
}

func TestClientCurrentOperationsReturnsCommandError(t *testing.T) {
	expected := errors.New("command failed")
	client := NewClient(&fakeScanCommandRunner{err: expected})

	_, err := client.CurrentOperations(context.Background())

	if !errors.Is(err, expected) {
		t.Fatalf("expected command error, got %v", err)
	}
}

type fakeScanCommandRunner struct {
	database    string
	commandName string
	verbosity   string
	currentOps  currentOpResult
	explain     explainResult
	err         error
}

func (r *fakeScanCommandRunner) RunCommand(_ context.Context, database string, command Command, out any) error {
	r.database = database
	r.commandName = command.Name

	if r.err != nil {
		return r.err
	}

	switch result := out.(type) {
	case *currentOpResult:
		*result = r.currentOps
	case *explainResult:
		r.verbosity, _ = command.Document["verbosity"].(string)
		*result = r.explain
	default:
		return errors.New("unexpected output type")
	}

	return nil
}
