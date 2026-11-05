# dev-workspaces

**Your editor workspaces, one command away.**

Stop browsing through File → Open Workspace. Register a folder or saved VS Code
workspace once, then open it from any terminal:

```sh
dev web
dev calendar
dev                  # Interactive picker: select a number or type to filter
```

## Install

Requires Python 3.11+ on macOS or Linux. Install with [uv](https://docs.astral.sh/uv/):

```sh
uv tool install git+https://github.com/filipbodor/dev-workspaces.git
```

Make sure uv's tool directory is on your PATH (`uv tool update-shell` can help).
VS Code's `code` command must also be available. On macOS, run **Shell Command:
Install 'code' command in PATH** from VS Code's Command Palette.

## Register your projects

```sh
dev add web ~/Projects/web/web.code-workspace
dev add calendar ~/Projects/calendar
dev add playground                         # Register the current folder
dev list
dev web
```

Saved `.code-workspace` files open as workspaces, including their configured
folders and settings. Folders open directly. The command opens the editor;
your terminal's current directory stays the same.

## Make it yours

```sh
dev editor code --new-window               # VS Code, in a new window
dev editor cursor                          # Or another compatible editor CLI
dev add web ~/Projects/new-web --replace   # Update a saved path
dev remove playground                      # Unregister; keep all project files
dev config                                 # Print the configuration location
```

Configuration stays on your machine at `~/.config/dev-workspaces/config.json`
(or under `XDG_CONFIG_HOME`). Nothing is uploaded. The editor is stored as an
argument list; shell aliases and shell syntax are not evaluated.

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
are reserved. Running `dev` without a name needs an interactive terminal;
scripts can use `dev NAME`.

## Development

```sh
uv venv
uv pip install -e . ruff build
.venv/bin/python -m unittest discover -s tests -v
.venv/bin/ruff check .
.venv/bin/ruff format --check .
.venv/bin/python -m build
```

Tests use temporary configurations and a fake editor. They never open your IDE.
Bug reports and focused contributions are welcome. [MIT licensed](LICENSE).
