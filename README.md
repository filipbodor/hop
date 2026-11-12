<p align="center">
  <img src="assets/logo.png" width="240" alt="Hop logo: a hopping rabbit in coral and violet">
</p>

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

Download the archive for your machine from [Releases](https://github.com/filipbodor/hop/releases/latest):
`darwin` means macOS, `arm64` means Apple Silicon or Linux ARM, and `amd64` means Intel/AMD.
Extract it and put `hop` on your PATH, for example:

```sh
tar -xzf hop_VERSION_darwin_arm64.tar.gz   # Use your downloaded filename
mkdir -p ~/.local/bin
install -m 755 hop ~/.local/bin/hop
```

No Python, Go, or other language runtime is needed. Add `~/.local/bin` to your
PATH if it isn't there already. Checksums are included with each release.

To verify your download before extracting it, download `checksums.txt` from the
same release into the archive's directory, then run:

```sh
shasum -a 256 --ignore-missing -c checksums.txt
```

Your downloaded archive should report `OK`. On Linux, you can use
`sha256sum --ignore-missing -c checksums.txt` instead.

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
hop add ios ~/Projects/App/App.xcworkspace --app xcode
hop add frontend ~/Projects/frontend --app cursor
hop add playground                         # Register the current folder
hop list
hop web
```

Saved `.code-workspace` files open with their configured folders and settings.
Xcode `.xcworkspace` and `.xcodeproj` bundles are supported too. Your terminal's
current directory stays the same.

`--app` overrides the default editor for one project. Use an editor CLI such as
`code`, `cursor`, or `idea`; on macOS, use `xcode` or an app name ending in `.app`,
such as `--app "Android Studio.app"`. App choices appear in the list and picker.

## Make it yours

```sh
hop editor code --new-window               # VS Code, in a new window
hop editor cursor                          # Or another compatible editor CLI
hop add web ~/Projects/new-web --replace   # Update a saved path
hop add ios ~/Projects/App/App.xcworkspace --app default --replace # Reset app
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
    "calendar": "/path/to/calendar",
    "ios": {"path": "/path/to/App.xcworkspace", "app": "xcode"}
  }
}
```

Plain paths keep using the default editor. Updating a path with `--replace`
preserves its app unless you also supply `--app`.

Names accept letters, numbers, hyphens and underscores. Management command names
are reserved. Running `hop` without a name needs an interactive terminal;
scripts can use `hop NAME`.

**Upgrading from dev-workspaces:** Hop reuses your existing
`~/.config/dev-workspaces/config.json` when no Hop config exists. Run `uv tool uninstall
dev-workspaces` to remove the old Python command; your config stays in place.
If you want to keep `dev` as a shortcut, create a symlink to the installed `hop` binary.

## Use with coding agents

The optional [hop-setup skill](skills/hop-setup/SKILL.md) helps an agent register
projects, choose apps, and update shortcuts while preserving your other settings.
From a cloned copy of this repository, install it for Codex:

```sh
mkdir -p ~/.codex/skills
cp -R skills/hop-setup ~/.codex/skills/
```

For Claude Code, use `~/.claude/skills/` instead. Then ask your agent to configure
Hop for your projects. The Hop executable must be installed separately.

## Worktree sessions

Git worktrees are separate checkout directories sharing one repository's history.
Each can keep a different branch and its own uncommitted changes. Hop can discover
these checkouts and open a selected one across your tools:

```sh
hop worktree --list
hop worktree feature-name
hop worktree --repo /path/to/repository feature-name
```

Discovery uses Git's worktree metadata from the current repository and registered
project folders, including local folders listed in JSON `.code-workspace` files.
Linked worktrees are found even when an agent placed them elsewhere. No recursive
filesystem scan, repository tasks, or server startup scripts are executed. The
picker shows repository, branch (or detached HEAD), path, and missing/locked state.
Malformed workspace files are reported and skipped. Workspace URIs and JSONC
comments are not interpreted; register an ordinary repository folder if needed.

To also move the **current terminal** into the selected worktree, add this once
near the end of `.zshrc`:

```sh
eval "$(hop init zsh)"
```

Then open a new terminal and use:

```sh
hwt                       # Pick, open editor + Sublime Merge, and cd
hwt project review        # Filter by repository, branch, and/or path fragments
hwt --list                # List only; no apps or directory change
hwt --path-only review    # Pick and cd without launching apps
```

The configured default editor is used, with a new window for VS Code/Cursor.
Sublime Merge uses `smerge` on PATH or the standard macOS app installation. Both
commands receive the same directory and run with it as their working directory.
The current terminal then lands there, ready for your usual server command.
Dependencies and server ports remain per-worktree concerns; Hop does not start or
stop servers, switch branches, create, prune, or delete worktrees.

Multiple matches require an interactive terminal; add more search words for
scripts. `hop worktree --path-only` prints only the selected absolute path, with
picker UI on stderr; cancellation emits no path. Explicit project registrations
named `init` or `worktree` from an older configuration remain accessible through
`hop open NAME`; new registrations reserve these command names.

## Troubleshooting

- **Editor not found:** check that the editor command is on your terminal's PATH
  (for example, `command -v code`). Set the default with `hop editor code`, or
  update a project's override with
  `hop add NAME /path/to/project --app cursor --replace`.
- **Project folder moved:** run `hop list` to check the saved path, then update it
  with `hop add NAME /new/path --replace`. The project's app choice is preserved.
- **Unexpected configuration:** run `hop config` to see which file Hop is using.
  Check `XDG_CONFIG_HOME` and the legacy dev-workspaces fallback described above.

## Development

```sh
go test -race ./...
go vet ./...
go build -o build/hop ./cmd/hop
./build/hop --help
./scripts/release.sh v0.3.0                # Build macOS/Linux archives and checksums
```

Tests use temporary configurations and a fake editor. They never open your IDE.
Tag pushes run tests, build release archives, and publish a GitHub release.
Bug reports and focused contributions are welcome. [MIT licensed](LICENSE).
