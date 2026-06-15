package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/RostyslavPaliuha/mongoDb-performance-troubleshooter/internal/mongodb"
)

const usageText = `MongoDB Performance Troubleshooter (mpt)

Usage:
  mpt [--help]
  mpt --version [--uri <mongodb-uri>]
  mpt version [--uri <mongodb-uri>]

Options:
  -h, --help            Show help.
  --version             Show MongoDB server version.
  --uri <mongodb-uri>   MongoDB URI. Defaults to MONGODB_URI or mongodb://localhost:27017.
`

const defaultMongoDBURI = "mongodb://localhost:27017"

type mongoClient interface {
	ServerVersion(context.Context) (mongodb.Version, error)
	Disconnect(context.Context) error
}

type Dependencies struct {
	Connect func(context.Context, mongodb.Config) (mongoClient, error)
	Getenv  func(string) string
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
	case "version", "--version":
		return runVersion(args[1:], stdout, stderr, deps)
	default:
		fmt.Fprintf(stderr, "unknown argument: %s\n\n%s", args[0], usageText)
		return 1
	}
}

func defaultDependencies() Dependencies {
	return Dependencies{
		Connect: func(ctx context.Context, config mongodb.Config) (mongoClient, error) {
			return mongodb.Connector{}.Connect(ctx, config)
		},
		Getenv: os.Getenv,
	}
}

func normalizeDependencies(deps Dependencies) Dependencies {
	defaults := defaultDependencies()
	if deps.Connect == nil {
		deps.Connect = defaults.Connect
	}
	if deps.Getenv == nil {
		deps.Getenv = defaults.Getenv
	}
	return deps
}

func runVersion(args []string, stdout, stderr io.Writer, deps Dependencies) int {
	uri, err := versionURI(args, deps)
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

func versionURI(args []string, deps Dependencies) (string, error) {
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
