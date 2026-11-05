# Hop

- Keep the Go version in go.mod supported. Release binaries must run without a language runtime.
- User paths and editor preferences belong in private config, never in this repository.
- Launch editors with argument lists, never shell evaluation. Do not execute workspace contents.
- Preserve registered projects and require `--replace` before overwriting a name.
- Preserve the legacy dev-workspaces config fallback when changing configuration handling.
- Run `gofmt`, `go vet ./...`, and `go test -race ./...`.
- Build and smoke-test release archives when changing packaging or entry points.
