package mongodb

import (
	"context"
	"errors"
	"testing"
)

func TestClientServerVersionRunsBuildInfo(t *testing.T) {
	runner := &fakeCommandRunner{version: "3.6.23"}
	client := NewClient(runner)

	version, err := client.ServerVersion(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if version.String() != "3.6.23" {
		t.Fatalf("unexpected version: %s", version)
	}
	if runner.database != "admin" {
		t.Fatalf("expected admin database, got %q", runner.database)
	}
	if runner.commandName != "buildInfo" {
		t.Fatalf("expected buildInfo command, got %q", runner.commandName)
	}
}

func TestClientServerVersionReturnsCommandError(t *testing.T) {
	expected := errors.New("command failed")
	client := NewClient(&fakeCommandRunner{err: expected})

	_, err := client.ServerVersion(context.Background())

	if !errors.Is(err, expected) {
		t.Fatalf("expected command error, got %v", err)
	}
}

func TestClientServerVersionRejectsMalformedBuildInfoVersion(t *testing.T) {
	client := NewClient(&fakeCommandRunner{version: "broken"})

	_, err := client.ServerVersion(context.Background())

	if err == nil {
		t.Fatal("expected error")
	}
}

type fakeCommandRunner struct {
	database    string
	commandName string
	version     string
	err         error
}

func (r *fakeCommandRunner) RunCommand(_ context.Context, database string, command Command, out any) error {
	r.database = database
	r.commandName = command.Name

	if r.err != nil {
		return r.err
	}

	result, ok := out.(*buildInfoResult)
	if !ok {
		return errors.New("unexpected output type")
	}
	result.Version = r.version
	return nil
}
