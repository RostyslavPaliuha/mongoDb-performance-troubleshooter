# MongoDB Performance Troubleshooter

`mpt` is a command-line tool for investigating MongoDB performance issues.

The project is at the initial repository setup stage. MongoDB analysis features
will be added after the CLI foundation is in place.

## Install from source

```bash
go install github.com/RostyslavPaliuha/mongoDb-performance-troubleshooter/cmd/mpt@latest
```

## Usage

```bash
mpt --help
mpt -v
mpt -dbVersion --uri mongodb://localhost:27017
mpt scan --uri mongodb://localhost:27017
mpt scan --duration 10m --queryTime 30ms --output report.html
```

`mpt scan` samples live MongoDB operations for slow read queries, explains safe
read operations with execution stats, and writes a standalone HTML report with
the slow query, bad execution statistics, likely reason, and suggested fix.

Scan defaults:

- `--duration`: `1m`
- `--queryTime`: `50ms`
- `--output`: `mpt-scan-report-<timestamp>.html` in the current directory

## Development

```bash
go fmt ./...
go test ./...
go vet ./...
```
