package cli

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/RostyslavPaliuha/mongoDb-performance-troubleshooter/internal/mongodb"
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

func (c fakeMongoClient) Disconnect(context.Context) error {
	return nil
}
