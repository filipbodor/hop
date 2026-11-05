# Hop

**Your editor workspaces, one command away.**

Stop browsing through File → Open Workspace. Register a folder or saved VS Code
workspace once, then open it from any terminal:

```sh
hop web
hop calendar
hop                  # Interactive picker: select a number or type to filter
```

## Install

With Go 1.26+ installed, you can also build and install directly:

```sh
go install github.com/filipbodor/hop/cmd/hop@latest
```

Go installs to `$(go env GOPATH)/bin` by default; that directory must be on your PATH.
VS Code's `code` command must also be available. On macOS, run **Shell Command:
Install 'code' command in PATH** from VS Code's Command Palette.

## Register your projects

```sh
hop add web ~/Projects/web/web.code-workspace
hop add calendar ~/Projects/calendar
hop add playground                         # Register the current folder
hop list
hop web
```

Saved `.code-workspace` files open with their configured folders and settings.
Folders open directly. Your terminal's current directory stays the same.

## Make it yours

```sh
hop editor code --new-window               # VS Code, in a new window
hop editor cursor                          # Or another compatible editor CLI
hop add web ~/Projects/new-web --replace   # Update a saved path
hop remove playground                      # Unregister; keep project files
hop config                                 # Print the configuration location
```

Configuration stays on your machine at `~/.config/hop/config.json` (or under
`XDG_CONFIG_HOME`). Nothing is uploaded. The editor is stored as an argument
list; shell aliases and shell syntax are not evaluated.

```json
{
  "editor": ["code", "--new-window"],
  "projects": {
    "web": "/path/to/web.code-workspace",
    "calendar": "/path/to/calendar"
  }
}
```

Names accept letters, numbers, hyphens and underscores. Management command names
are reserved. Running `hop` without a name needs an interactive terminal;
scripts can use `hop NAME`.

**Upgrading from dev-workspaces:** Hop reuses your existing
`~/.config/dev-workspaces/config.json` when no Hop config exists. Run `uv tool uninstall
dev-workspaces` to remove the old Python command; your config stays in place.
If you want to keep `dev` as a shortcut, create a symlink to the installed `hop` binary.

## Development

```sh
go test -race ./...
go vet ./...
go build -o build/hop ./cmd/hop
./build/hop --help
```

Tests use temporary configurations and a fake editor. They never open your IDE.
Bug reports and focused contributions are welcome. [MIT licensed](LICENSE).
