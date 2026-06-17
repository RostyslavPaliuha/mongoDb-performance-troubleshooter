package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"
	"time"

	"github.com/RostyslavPaliuha/mongoDb-performance-troubleshooter/internal/mongodb"
	"github.com/RostyslavPaliuha/mongoDb-performance-troubleshooter/internal/scan"
)

func TestRunWithoutArgsPrintsHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if stdout.String() != usageText {
		t.Fatalf("expected help text on stdout, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunHelpFlagPrintsHelp(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-h"}} {
		t.Run(args[0], func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			code := Run(args, &stdout, &stderr)

			if code != 0 {
				t.Fatalf("expected exit code 0, got %d", code)
			}
			if stdout.String() != usageText {
				t.Fatalf("expected help text on stdout, got %q", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected empty stderr, got %q", stderr.String())
			}
		})
	}
}

func TestRunToolVersionFlagPrintsToolVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	deps := Dependencies{
		Connect: func(context.Context, mongodb.Config) (mongoClient, error) {
			t.Fatal("tool version must not connect to MongoDB")
			return nil, nil
		},
	}

	code := RunWithDependencies([]string{"-v"}, &stdout, &stderr, deps)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if stdout.String() != "mpt dev\n" {
		t.Fatalf("expected version on stdout, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunToolVersionFlagPrintsInjectedToolVersion(t *testing.T) {
	previousVersion := version
	version = "v0.1.0"
	t.Cleanup(func() {
		version = previousVersion
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"-v"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if stdout.String() != "mpt v0.1.0\n" {
		t.Fatalf("expected injected version on stdout, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestToolVersionUsesTaggedModuleVersion(t *testing.T) {
	info := debug.BuildInfo{
		Main: debug.Module{Version: "v0.1.0"},
	}

	if got := toolVersion(info); got != "v0.1.0" {
		t.Fatalf("expected tagged module version, got %q", got)
	}
}

func TestToolVersionFallsBackToDevForLocalBuild(t *testing.T) {
	info := debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
	}

	if got := toolVersion(info); got != "dev" {
		t.Fatalf("expected dev version, got %q", got)
	}
}

func TestRunDatabaseVersionFlagPrintsMongoDBServerVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	deps := Dependencies{
		Connect: fakeConnector{
			client: fakeMongoClient{version: mongodb.Version{Major: 7, Minor: 0, Patch: 12}},
		}.Connect,
	}

	code := RunWithDependencies([]string{"-dbVersion", "--uri", "mongodb://localhost:27017"}, &stdout, &stderr, deps)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if stdout.String() != "MongoDB server version: 7.0.12\n" {
		t.Fatalf("expected MongoDB server version on stdout, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunDatabaseVersionFlagPrintsConnectionError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	deps := Dependencies{
		Connect: fakeConnector{err: errors.New("connection refused")}.Connect,
	}

	code := RunWithDependencies([]string{"-dbVersion", "--uri", "mongodb://localhost:27017"}, &stdout, &stderr, deps)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	if stderr.String() != "get MongoDB server version: connect to MongoDB: connection refused\n" {
		t.Fatalf("expected connection error on stderr, got %q", stderr.String())
	}
}

func TestRunScanWritesReportToExplicitOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	outputPath := filepath.Join(t.TempDir(), "report.html")

	deps := Dependencies{
		Connect: fakeConnector{
			client: fakeMongoClient{},
		}.Connect,
		Scan: func(_ context.Context, _ mongoClient, options ScanOptions) (scan.Report, error) {
			if options.Duration != 10*time.Minute {
				t.Fatalf("expected 10m duration, got %s", options.Duration)
			}
			if options.QueryTime != 30*time.Millisecond {
				t.Fatalf("expected 30ms query time, got %s", options.QueryTime)
			}
			return scan.Report{
				StartedAt: time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC),
				EndedAt:   time.Date(2026, 6, 15, 10, 10, 0, 0, time.UTC),
				Duration:  options.Duration,
				QueryTime: options.QueryTime,
			}, nil
		},
	}

	code := RunWithDependencies([]string{
		"scan",
		"--uri", "mongodb://localhost:27017",
		"--duration", "10m",
		"--queryTime", "30ms",
		"--output", outputPath,
	}, &stdout, &stderr, deps)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", code, stderr.String())
	}
	if stdout.String() != "HTML report written to "+outputPath+"\n" {
		t.Fatalf("expected report path on stdout, got %q", stdout.String())
	}
	report, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("expected report file: %v", err)
	}
	if !bytes.Contains(report, []byte("MongoDB Performance Troubleshooter Report")) {
		t.Fatalf("expected report HTML, got %q", string(report))
	}
}

func TestRunScanUsesDefaultDurationThresholdAndGeneratedOutput(t *testing.T) {
	t.Chdir(t.TempDir())
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	deps := Dependencies{
		Connect: fakeConnector{
			client: fakeMongoClient{},
		}.Connect,
		Scan: func(_ context.Context, _ mongoClient, options ScanOptions) (scan.Report, error) {
			if options.Duration != time.Minute {
				t.Fatalf("expected default 1m duration, got %s", options.Duration)
			}
			if options.QueryTime != 50*time.Millisecond {
				t.Fatalf("expected default 50ms query time, got %s", options.QueryTime)
			}
			return scan.Report{
				StartedAt: time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC),
				EndedAt:   time.Date(2026, 6, 15, 10, 1, 0, 0, time.UTC),
				Duration:  options.Duration,
				QueryTime: options.QueryTime,
			}, nil
		},
	}

	code := RunWithDependencies([]string{"scan", "--uri", "mongodb://localhost:27017"}, &stdout, &stderr, deps)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", code, stderr.String())
	}
	matches, err := filepath.Glob("mpt-scan-report-*.html")
	if err != nil {
		t.Fatalf("glob generated reports: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one generated report, got %#v", matches)
	}
	if stdout.String() != "HTML report written to "+matches[0]+"\n" {
		t.Fatalf("expected generated report path on stdout, got %q", stdout.String())
	}
}

func TestRunScanRejectsInvalidQueryTime(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	deps := Dependencies{
		Connect: func(context.Context, mongodb.Config) (mongoClient, error) {
			t.Fatal("invalid query time must not connect to MongoDB")
			return nil, nil
		},
	}

	code := RunWithDependencies([]string{"scan", "--queryTime", "0ms"}, &stdout, &stderr, deps)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	if stderr.String() != "--queryTime must be greater than zero\n" {
		t.Fatalf("expected validation error, got %q", stderr.String())
	}
}

func TestRunUnknownArgumentPrintsError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--unknown"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	if stderr.String() != "unknown argument: --unknown\n\n"+usageText {
		t.Fatalf("expected error and help on stderr, got %q", stderr.String())
	}
}

type fakeConnector struct {
	client fakeMongoClient
	err    error
}

func (c fakeConnector) Connect(_ context.Context, config mongodb.Config) (mongoClient, error) {
	if c.err != nil {
		return nil, c.err
	}
	if config.URI != "mongodb://localhost:27017" {
		return nil, errors.New("unexpected URI")
	}
	return c.client, nil
}

type fakeMongoClient struct {
	version mongodb.Version
}

func (c fakeMongoClient) ServerVersion(context.Context) (mongodb.Version, error) {
	return c.version, nil
}

func (c fakeMongoClient) CurrentOperations(context.Context) ([]mongodb.ScanOperation, error) {
	return nil, nil
}

func (c fakeMongoClient) ExplainOperation(context.Context, mongodb.ScanOperation) (mongodb.ScanExecutionStats, error) {
	return mongodb.ScanExecutionStats{}, nil
}

func (c fakeMongoClient) Disconnect(context.Context) error {
	return nil
}
