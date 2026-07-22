# AWS Lambda Handler

Provides a wrapper method (with logging and XRay tracing) to start a lambda function.

> [!IMPORTANT]
> This repository does not use semantic versioning

## Usage:

See [examples/handler/main.go](./examples/handler/main.go) and [examples/sqs/main.go](./examples/sqs/main.go)

## Tasks

[xcfile.dev](https://xcfile.dev/) tasks

### test

```shell
#!/bin/bash
go test ./... -json | tparse
```

### format

```shell
go fmt $(go list ./...)
```

### sast

```shell
wget -O .golangci.json https://raw.githubusercontent.com/ockendenjo/actions/refs/heads/main/.golangci.json
golangci-lint run
govulncheck ./...
```

### vet

```shell
#!/bin/bash
go vet ./...
```
