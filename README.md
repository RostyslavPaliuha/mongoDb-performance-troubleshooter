# MongoDB Performance Troubleshooter

`mpt` is a command-line tool for investigating MongoDB performance issues.

The project is at the initial repository setup stage. MongoDB analysis features
will be added after the CLI foundation is in place.

## Install from source

```bash
go install github.com/rostyslavpaliuha/mongodb-performance-troubleshooter/cmd/mpt@latest
```

## Usage

```bash
mpt --help
mpt --version
```

## Development

```bash
go fmt ./...
go test ./...
go vet ./...
```
