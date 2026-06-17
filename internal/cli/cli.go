package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/RostyslavPaliuha/mongoDb-performance-troubleshooter/internal/mongodb"
	"github.com/RostyslavPaliuha/mongoDb-performance-troubleshooter/internal/scan"
)

const usageText = `MongoDB Performance Troubleshooter (mpt)

Usage:
  mpt [--help]
  mpt -v
  mpt -dbVersion [--uri <mongodb-uri>]
  mpt scan [--uri <mongodb-uri>] [--duration <duration>] [--queryTime <duration>] [--output <path>]

Options:
  -h, --help              Show help.
  -v                      Show mpt version.
  -dbVersion              Show MongoDB server version.
  scan                    Scan live slow read operations and write an HTML report.
  --uri <mongodb-uri>     MongoDB URI. Defaults to MONGODB_URI or mongodb://localhost:27017.
  --duration <duration>   Scan duration. Defaults to 1m.
  --queryTime <duration>  Slow query threshold. Defaults to 50ms.
  --output <path>         HTML report path. Defaults to mpt-scan-report-<timestamp>.html.
`

const defaultMongoDBURI = "mongodb://localhost:27017"
const defaultScanDuration = time.Minute
const defaultScanQueryTime = 50 * time.Millisecond

var version = "dev"

type mongoClient interface {
	ServerVersion(context.Context) (mongodb.Version, error)
	CurrentOperations(context.Context) ([]mongodb.ScanOperation, error)
	ExplainOperation(context.Context, mongodb.ScanOperation) (mongodb.ScanExecutionStats, error)
	Disconnect(context.Context) error
}

type Dependencies struct {
	Connect func(context.Context, mongodb.Config) (mongoClient, error)
	Scan    func(context.Context, mongoClient, ScanOptions) (scan.Report, error)
	Getenv  func(string) string
}

type ScanOptions struct {
	Duration  time.Duration
	QueryTime time.Duration
}

func Run(args []string, stdout, stderr io.Writer) int {
	return RunWithDependencies(args, stdout, stderr, defaultDependencies())
}

func RunWithDependencies(args []string, stdout, stderr io.Writer, deps Dependencies) int {
	deps = normalizeDependencies(deps)

	if len(args) == 0 {
		fmt.Fprint(stdout, usageText)
		return 0
	}

	switch args[0] {
	case "-h", "--help":
		fmt.Fprint(stdout, usageText)
		return 0
	case "-v":
		fmt.Fprintf(stdout, "mpt %s\n", Version())
		return 0
	case "-dbVersion":
		return runDatabaseVersion(args[1:], stdout, stderr, deps)
	case "scan":
		return runScan(args[1:], stdout, stderr, deps)
	default:
		fmt.Fprintf(stderr, "unknown argument: %s\n\n%s", args[0], usageText)
		return 1
	}
}

func Version() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		return toolVersion(*info)
	}
	return version
}

func toolVersion(info debug.BuildInfo) string {
	if version != "dev" {
		return version
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

func defaultDependencies() Dependencies {
	return Dependencies{
		Connect: func(ctx context.Context, config mongodb.Config) (mongoClient, error) {
			return mongodb.Connector{}.Connect(ctx, config)
		},
		Scan:   runDefaultScan,
		Getenv: os.Getenv,
	}
}

func normalizeDependencies(deps Dependencies) Dependencies {
	defaults := defaultDependencies()
	if deps.Connect == nil {
		deps.Connect = defaults.Connect
	}
	if deps.Scan == nil {
		deps.Scan = defaults.Scan
	}
	if deps.Getenv == nil {
		deps.Getenv = defaults.Getenv
	}
	return deps
}

func runDatabaseVersion(args []string, stdout, stderr io.Writer, deps Dependencies) int {
	uri, err := databaseVersionURI(args, deps)
	if err != nil {
		fmt.Fprintf(stderr, "%s\n", err)
		return 1
	}

	config, err := mongodb.NewConfig(uri)
	if err != nil {
		fmt.Fprintf(stderr, "%s\n", err)
		return 1
	}
	config.ConnectTimeout = 10 * time.Second

	ctx := context.Background()
	client, err := deps.Connect(ctx, config)
	if err != nil {
		fmt.Fprintf(stderr, "get MongoDB server version: connect to MongoDB: %s\n", err)
		return 1
	}
	defer func() {
		_ = client.Disconnect(context.Background())
	}()

	version, err := client.ServerVersion(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "get MongoDB server version: %s\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "MongoDB server version: %s\n", version)
	return 0
}

func databaseVersionURI(args []string, deps Dependencies) (string, error) {
	uri := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--uri":
			if i+1 >= len(args) {
				return "", fmt.Errorf("--uri requires a value")
			}
			uri = args[i+1]
			i++
		default:
			return "", fmt.Errorf("unknown version argument: %s", args[i])
		}
	}

	if uri != "" {
		return uri, nil
	}
	if deps.Getenv != nil {
		uri = deps.Getenv("MONGODB_URI")
	}
	if uri != "" {
		return uri, nil
	}
	return defaultMongoDBURI, nil
}

func runScan(args []string, stdout, stderr io.Writer, deps Dependencies) int {
	parsed, err := parseScanArgs(args, deps)
	if err != nil {
		fmt.Fprintf(stderr, "%s\n", err)
		return 1
	}

	config, err := mongodb.NewConfig(parsed.uri)
	if err != nil {
		fmt.Fprintf(stderr, "%s\n", err)
		return 1
	}
	config.ConnectTimeout = 10 * time.Second

	ctx := context.Background()
	client, err := deps.Connect(ctx, config)
	if err != nil {
		fmt.Fprintf(stderr, "scan MongoDB slow queries: connect to MongoDB: %s\n", err)
		return 1
	}
	defer func() {
		_ = client.Disconnect(context.Background())
	}()

	report, err := deps.Scan(ctx, client, ScanOptions{
		Duration:  parsed.duration,
		QueryTime: parsed.queryTime,
	})
	if err != nil {
		fmt.Fprintf(stderr, "scan MongoDB slow queries: %s\n", err)
		return 1
	}

	html, err := scan.RenderHTML(report)
	if err != nil {
		fmt.Fprintf(stderr, "render HTML report: %s\n", err)
		return 1
	}

	outputPath := parsed.outputPath
	if outputPath == "" {
		outputPath = defaultScanOutputPath(time.Now())
	}
	if err := os.WriteFile(outputPath, []byte(html), 0o644); err != nil {
		fmt.Fprintf(stderr, "write HTML report: %s\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "HTML report written to %s\n", outputPath)
	return 0
}

type parsedScanArgs struct {
	uri        string
	duration   time.Duration
	queryTime  time.Duration
	outputPath string
}

func parseScanArgs(args []string, deps Dependencies) (parsedScanArgs, error) {
	parsed := parsedScanArgs{
		duration:  defaultScanDuration,
		queryTime: defaultScanQueryTime,
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--uri":
			if i+1 >= len(args) {
				return parsedScanArgs{}, fmt.Errorf("--uri requires a value")
			}
			parsed.uri = args[i+1]
			i++
		case "--duration":
			if i+1 >= len(args) {
				return parsedScanArgs{}, fmt.Errorf("--duration requires a value")
			}
			duration, err := time.ParseDuration(args[i+1])
			if err != nil {
				return parsedScanArgs{}, fmt.Errorf("--duration must be a valid duration")
			}
			if duration <= 0 {
				return parsedScanArgs{}, fmt.Errorf("--duration must be greater than zero")
			}
			parsed.duration = duration
			i++
		case "--queryTime":
			if i+1 >= len(args) {
				return parsedScanArgs{}, fmt.Errorf("--queryTime requires a value")
			}
			queryTime, err := time.ParseDuration(args[i+1])
			if err != nil {
				return parsedScanArgs{}, fmt.Errorf("--queryTime must be a valid duration")
			}
			if queryTime <= 0 {
				return parsedScanArgs{}, fmt.Errorf("--queryTime must be greater than zero")
			}
			parsed.queryTime = queryTime
			i++
		case "--output":
			if i+1 >= len(args) {
				return parsedScanArgs{}, fmt.Errorf("--output requires a value")
			}
			parsed.outputPath = args[i+1]
			i++
		default:
			return parsedScanArgs{}, fmt.Errorf("unknown scan argument: %s", args[i])
		}
	}

	if parsed.uri == "" && deps.Getenv != nil {
		parsed.uri = deps.Getenv("MONGODB_URI")
	}
	if parsed.uri == "" {
		parsed.uri = defaultMongoDBURI
	}

	return parsed, nil
}

func defaultScanOutputPath(now time.Time) string {
	return filepath.Join(".", fmt.Sprintf("mpt-scan-report-%s.html", now.Format("20060102-150405")))
}

func runDefaultScan(ctx context.Context, client mongoClient, options ScanOptions) (scan.Report, error) {
	return scan.Scanner{
		Provider:     scanClientAdapter{client: client},
		Duration:     options.Duration,
		QueryTime:    options.QueryTime,
		PollInterval: time.Second,
	}.Scan(ctx)
}

type scanClientAdapter struct {
	client mongoClient
}

func (a scanClientAdapter) CurrentOperations(ctx context.Context) ([]scan.Operation, error) {
	operations, err := a.client.CurrentOperations(ctx)
	if err != nil {
		return nil, err
	}

	scanOperations := make([]scan.Operation, 0, len(operations))
	for _, operation := range operations {
		scanOperations = append(scanOperations, scan.Operation{
			ID:               operation.ID,
			Namespace:        operation.Namespace,
			Operation:        operation.Operation,
			MicrosecsRunning: operation.MicrosecsRunning,
			Command:          operation.Command,
		})
	}
	return scanOperations, nil
}

func (a scanClientAdapter) ExplainOperation(ctx context.Context, operation scan.Operation) (scan.ExecutionStats, error) {
	stats, err := a.client.ExplainOperation(ctx, mongodb.ScanOperation{
		ID:               operation.ID,
		Namespace:        operation.Namespace,
		Operation:        operation.Operation,
		MicrosecsRunning: operation.MicrosecsRunning,
		Command:          operation.Command,
	})
	if err != nil {
		return scan.ExecutionStats{}, err
	}

	return scan.ExecutionStats{
		Stage:             stats.Stage,
		TotalDocsExamined: stats.TotalDocsExamined,
		TotalKeysExamined: stats.TotalKeysExamined,
		NReturned:         stats.NReturned,
		HasBlockingSort:   stats.HasBlockingSort,
	}, nil
}
