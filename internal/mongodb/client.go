package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

const MinSupportedVersion = "3.6"

type Config struct {
	URI            string
	ConnectTimeout time.Duration
}

type Command struct {
	Name     string
	Document map[string]any
}

type CommandRunner interface {
	RunCommand(ctx context.Context, database string, command Command, out any) error
}

type Connector struct{}

type Client struct {
	runner      CommandRunner
	mongoClient *mongo.Client
}

type driverCommandRunner struct {
	client *mongo.Client
}

type buildInfoResult struct {
	Version string `bson:"version"`
}

type ScanOperation struct {
	ID               string
	Namespace        string
	Operation        string
	MicrosecsRunning int64
	Command          map[string]any
}

type ScanExecutionStats struct {
	Stage             string
	TotalDocsExamined int64
	TotalKeysExamined int64
	NReturned         int64
	HasBlockingSort   bool
}

type currentOpResult struct {
	Inprog []currentOpDocument `bson:"inprog"`
}

type currentOpDocument struct {
	OpID             any            `bson:"opid"`
	Ns               string         `bson:"ns"`
	Op               string         `bson:"op"`
	MicrosecsRunning int64          `bson:"microsecs_running"`
	SecsRunning      int64          `bson:"secs_running"`
	Command          map[string]any `bson:"command"`
}

type explainResult struct {
	ExecutionStats executionStatsDocument `bson:"executionStats"`
}

type executionStatsDocument struct {
	NReturned         int64                  `bson:"nReturned"`
	TotalDocsExamined int64                  `bson:"totalDocsExamined"`
	TotalKeysExamined int64                  `bson:"totalKeysExamined"`
	ExecutionStages   executionStageDocument `bson:"executionStages"`
}

type executionStageDocument struct {
	Stage       string                   `bson:"stage"`
	InputStage  *executionStageDocument  `bson:"inputStage"`
	InputStages []executionStageDocument `bson:"inputStages"`
}

func NewConfig(uri string) (Config, error) {
	if uri == "" {
		return Config{}, errors.New("MongoDB URI is required")
	}

	return Config{
		URI:            uri,
		ConnectTimeout: 10 * time.Second,
	}, nil
}

func (Connector) Connect(ctx context.Context, config Config) (*Client, error) {
	if config.URI == "" {
		return nil, errors.New("MongoDB URI is required")
	}

	if config.ConnectTimeout <= 0 {
		config.ConnectTimeout = 10 * time.Second
	}

	connectCtx, cancel := context.WithTimeout(ctx, config.ConnectTimeout)
	defer cancel()

	mongoClient, err := mongo.Connect(connectCtx, options.Client().ApplyURI(config.URI))
	if err != nil {
		return nil, fmt.Errorf("connect to MongoDB: %w", err)
	}

	client := &Client{
		runner:      &driverCommandRunner{client: mongoClient},
		mongoClient: mongoClient,
	}

	if err := client.Ping(connectCtx); err != nil {
		_ = mongoClient.Disconnect(context.Background())
		return nil, fmt.Errorf("ping MongoDB: %w", err)
	}

	return client, nil
}

func NewClient(runner CommandRunner) *Client {
	return &Client{runner: runner}
}

func (c *Client) Ping(ctx context.Context) error {
	if c.mongoClient == nil {
		return nil
	}

	return c.mongoClient.Ping(ctx, readpref.Primary())
}

func (c *Client) Disconnect(ctx context.Context) error {
	if c.mongoClient == nil {
		return nil
	}

	return c.mongoClient.Disconnect(ctx)
}

func (c *Client) ServerVersion(ctx context.Context) (Version, error) {
	if c.runner == nil {
		return Version{}, errors.New("MongoDB command runner is not configured")
	}

	var result buildInfoResult
	if err := c.runner.RunCommand(ctx, "admin", Command{Name: "buildInfo"}, &result); err != nil {
		return Version{}, fmt.Errorf("get MongoDB buildInfo: %w", err)
	}

	version, err := ParseVersion(result.Version)
	if err != nil {
		return Version{}, fmt.Errorf("parse MongoDB server version: %w", err)
	}

	return version, nil
}

func (c *Client) CurrentOperations(ctx context.Context) ([]ScanOperation, error) {
	if c.runner == nil {
		return nil, errors.New("MongoDB command runner is not configured")
	}

	var result currentOpResult
	if err := c.runner.RunCommand(ctx, "admin", Command{
		Name: "currentOp",
		Document: map[string]any{
			"currentOp": true,
			"active":    true,
		},
	}, &result); err != nil {
		return nil, fmt.Errorf("get current MongoDB operations: %w", err)
	}

	operations := make([]ScanOperation, 0, len(result.Inprog))
	for _, operation := range result.Inprog {
		microsecsRunning := operation.MicrosecsRunning
		if microsecsRunning == 0 && operation.SecsRunning > 0 {
			microsecsRunning = operation.SecsRunning * int64(time.Second/time.Microsecond)
		}
		operations = append(operations, ScanOperation{
			ID:               fmt.Sprint(operation.OpID),
			Namespace:        operation.Ns,
			Operation:        operation.Op,
			MicrosecsRunning: microsecsRunning,
			Command:          operation.Command,
		})
	}

	return operations, nil
}

func (c *Client) ExplainOperation(ctx context.Context, operation ScanOperation) (ScanExecutionStats, error) {
	if c.runner == nil {
		return ScanExecutionStats{}, errors.New("MongoDB command runner is not configured")
	}
	if operation.Namespace == "" {
		return ScanExecutionStats{}, errors.New("operation namespace is required")
	}
	if len(operation.Command) == 0 {
		return ScanExecutionStats{}, errors.New("operation command is required")
	}

	var result explainResult
	if err := c.runner.RunCommand(ctx, databaseName(operation.Namespace), Command{
		Name: "explain",
		Document: map[string]any{
			"explain":   explainableCommand(operation.Command),
			"verbosity": "executionStats",
		},
	}, &result); err != nil {
		return ScanExecutionStats{}, fmt.Errorf("explain MongoDB operation: %w", err)
	}

	return ScanExecutionStats{
		Stage:             firstStage(result.ExecutionStats.ExecutionStages),
		TotalDocsExamined: result.ExecutionStats.TotalDocsExamined,
		TotalKeysExamined: result.ExecutionStats.TotalKeysExamined,
		NReturned:         result.ExecutionStats.NReturned,
		HasBlockingSort:   containsStage(result.ExecutionStats.ExecutionStages, "SORT"),
	}, nil
}

func (r *driverCommandRunner) RunCommand(ctx context.Context, database string, command Command, out any) error {
	if command.Name == "" {
		return errors.New("MongoDB command name is required")
	}

	result := r.client.Database(database).RunCommand(ctx, buildRunCommandDocument(command))
	if err := result.Decode(out); err != nil {
		return err
	}

	return nil
}

func buildRunCommandDocument(command Command) any {
	if command.Document == nil {
		return bson.D{{Key: command.Name, Value: 1}}
	}

	document := bson.D{{Key: command.Name, Value: command.Document[command.Name]}}
	for key, value := range command.Document {
		if key == command.Name {
			continue
		}
		document = append(document, bson.E{Key: key, Value: value})
	}
	return document
}

func databaseName(namespace string) string {
	for i, r := range namespace {
		if r == '.' {
			return namespace[:i]
		}
	}
	return namespace
}

func explainableCommand(command map[string]any) map[string]any {
	copied := make(map[string]any, len(command))
	for key, value := range command {
		switch key {
		case "$db", "lsid", "txnNumber", "$clusterTime", "readConcern":
			continue
		default:
			copied[key] = value
		}
	}
	return copied
}

func firstStage(stage executionStageDocument) string {
	if stage.Stage != "" {
		return stage.Stage
	}
	if stage.InputStage != nil {
		return firstStage(*stage.InputStage)
	}
	for _, child := range stage.InputStages {
		if childStage := firstStage(child); childStage != "" {
			return childStage
		}
	}
	return ""
}

func containsStage(stage executionStageDocument, target string) bool {
	if stage.Stage == target {
		return true
	}
	if stage.InputStage != nil && containsStage(*stage.InputStage, target) {
		return true
	}
	for _, child := range stage.InputStages {
		if containsStage(child, target) {
			return true
		}
	}
	return false
}
