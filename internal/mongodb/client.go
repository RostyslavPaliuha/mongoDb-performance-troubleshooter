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
	Name string
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

func (r *driverCommandRunner) RunCommand(ctx context.Context, database string, command Command, out any) error {
	if command.Name == "" {
		return errors.New("MongoDB command name is required")
	}

	result := r.client.Database(database).RunCommand(ctx, bson.D{{Key: command.Name, Value: 1}})
	if err := result.Decode(out); err != nil {
		return err
	}

	return nil
}
