package scan

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
)

type Provider interface {
	CurrentOperations(context.Context) ([]Operation, error)
	ExplainOperation(context.Context, Operation) (ExecutionStats, error)
}

type Scanner struct {
	Provider     Provider
	Duration     time.Duration
	QueryTime    time.Duration
	PollInterval time.Duration
	now          func() time.Time
	sleep        func(time.Duration)
}

type Operation struct {
	ID               string
	Namespace        string
	Operation        string
	MicrosecsRunning int64
	Command          map[string]any
}

type ExecutionStats struct {
	Stage             string
	TotalDocsExamined int64
	TotalKeysExamined int64
	NReturned         int64
	HasBlockingSort   bool
}

type Diagnostic struct {
	Reason string
	Fix    string
}

type Finding struct {
	Operation            Operation
	Stats                ExecutionStats
	Diagnostics          []Diagnostic
	ExplainSkippedReason string
	ExplainError         string
}

type Report struct {
	StartedAt time.Time
	EndedAt   time.Time
	Duration  time.Duration
	QueryTime time.Duration
	Findings  []Finding
}

func (s Scanner) Scan(ctx context.Context) (Report, error) {
	if s.Provider == nil {
		return Report{}, errors.New("scan provider is required")
	}
	if s.Duration <= 0 {
		return Report{}, errors.New("scan duration must be greater than zero")
	}
	if s.QueryTime <= 0 {
		return Report{}, errors.New("query time threshold must be greater than zero")
	}

	now := s.now
	if now == nil {
		now = time.Now
	}
	sleep := s.sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	pollInterval := s.PollInterval
	if pollInterval <= 0 {
		pollInterval = time.Second
	}

	startedAt := now()
	endsAt := startedAt.Add(s.Duration)
	report := Report{
		StartedAt: startedAt,
		Duration:  s.Duration,
		QueryTime: s.QueryTime,
	}
	findingsByID := map[string]int{}

	for {
		operations, err := s.Provider.CurrentOperations(ctx)
		if err != nil {
			return Report{}, fmt.Errorf("scan current MongoDB operations: %w", err)
		}

		for _, operation := range operations {
			if operationRuntime(operation) < s.QueryTime {
				continue
			}
			if _, exists := findingsByID[operation.ID]; exists {
				continue
			}

			finding := Finding{Operation: operation}
			if reason := explainSkipReason(operation); reason != "" {
				finding.ExplainSkippedReason = reason
			} else {
				stats, err := s.Provider.ExplainOperation(ctx, operation)
				if err != nil {
					finding.ExplainError = err.Error()
				} else {
					finding.Stats = stats
					finding.Diagnostics = Diagnose(stats)
				}
			}

			report.Findings = append(report.Findings, finding)
			findingsByID[operation.ID] = len(report.Findings) - 1
		}

		if !now().Before(endsAt) {
			break
		}
		sleep(pollInterval)
	}

	report.EndedAt = now()
	return report, nil
}

func operationRuntime(operation Operation) time.Duration {
	return time.Duration(operation.MicrosecsRunning) * time.Microsecond
}

func explainSkipReason(operation Operation) string {
	if _, ok := operation.Command["find"]; ok {
		return ""
	}

	if _, ok := operation.Command["aggregate"]; ok {
		if aggregateWrites(operation.Command["pipeline"]) {
			return "aggregate pipeline contains a write stage, so executionStats explain was skipped"
		}
		return ""
	}

	return "operation is not a supported read command for executionStats explain"
}

func aggregateWrites(pipeline any) bool {
	value := reflect.ValueOf(pipeline)
	if !value.IsValid() || value.Kind() != reflect.Slice {
		return false
	}

	for i := 0; i < value.Len(); i++ {
		stage := value.Index(i)
		if stage.Kind() == reflect.Interface {
			stage = stage.Elem()
		}
		if !stage.IsValid() || stage.Kind() != reflect.Map {
			continue
		}

		for _, key := range stage.MapKeys() {
			if key.Kind() != reflect.String {
				continue
			}
			if key.String() == "$out" || key.String() == "$merge" {
				return true
			}
		}
	}
	return false
}

func Diagnose(stats ExecutionStats) []Diagnostic {
	var diagnostics []Diagnostic

	if strings.EqualFold(stats.Stage, "COLLSCAN") {
		diagnostics = append(diagnostics, Diagnostic{
			Reason: "Collection scan examined every document.",
			Fix:    "Create an index that supports the query filter and sort fields.",
		})
	}

	if stats.NReturned > 0 && stats.TotalDocsExamined > stats.NReturned*100 {
		diagnostics = append(diagnostics, Diagnostic{
			Reason: "The query examined far more documents than it returned.",
			Fix:    "Add or adjust a selective index for the filter fields, and avoid predicates that cannot use an index.",
		})
	}

	if stats.NReturned > 0 && stats.TotalKeysExamined > stats.NReturned*100 {
		diagnostics = append(diagnostics, Diagnostic{
			Reason: "The query scanned far more index keys than returned documents.",
			Fix:    "Review compound index field order so equality, sort, and range predicates match the query shape.",
		})
	}

	if stats.HasBlockingSort {
		diagnostics = append(diagnostics, Diagnostic{
			Reason: "The execution plan contains a blocking sort.",
			Fix:    "Create a compound index that covers the filter fields followed by the sort fields.",
		})
	}

	if len(diagnostics) == 0 {
		diagnostics = append(diagnostics, Diagnostic{
			Reason: "No obvious bad execution statistic was detected.",
			Fix:    "Compare the query shape, data volume, and index selectivity with production expectations.",
		})
	}

	return diagnostics
}
