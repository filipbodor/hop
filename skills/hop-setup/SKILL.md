---
name: hop-setup
description: Configure Hop project shortcuts, workspace paths, and per-project apps. Use when a user wants to set up Hop, register or update a project, or fix a shortcut opening the wrong workspace or editor.
---

# Configure Hop

Use the installed `hop` CLI to manage the user's local shortcuts. If it is missing,
follow the [installation instructions](https://github.com/filipbodor/hop#install)
for their platform. The skill does not install the executable itself.

Inspect `hop --version`, `hop list`, and `hop config` before changing entries.
Use the configuration location reported by Hop: it respects `XDG_CONFIG_HOME`
and may reuse a legacy `dev-workspaces/config.json` file.

Choose the actual folder or saved workspace the user wants. For multi-repository
VS Code setups, prefer the existing `.code-workspace` file. For Xcode, use the
intended `.xcworkspace` or `.xcodeproj` bundle, not an embedded
`.xcodeproj/project.xcworkspace`. Inspect workspace references when ambiguous.
Ask only for a missing choice that cannot be inferred from the request or files.

Register projects through the CLI, quoting paths containing spaces:

```sh
hop add web "/path/to/web.code-workspace"
hop add ios "/path/to/App.xcworkspace" --app xcode
hop add frontend "/path/to/frontend" --app cursor
```

- Omit `--app` to use the global editor. Other app values name an executable,
  such as `code`, `cursor`, or `idea`; they are not shell command strings.
- On macOS, `xcode` and names ending in `.app` use the native app launcher,
  for example `--app "Android Studio.app"`. These choices require macOS.
- For an existing name, use `--replace` when the user requested that update.
  It preserves the saved app unless `--app` is supplied. Use
  `--app default --replace` to restore the global editor for that project.
- `hop editor COMMAND [ARGS]` changes the default for every project without an
  override. Use it only for a requested global change, not a single shortcut.
- `hop remove NAME` unregisters a shortcut; it does not delete project files.

Keep actual project names, paths, and app preferences in the user's private
configuration. Prefer CLI updates over rewriting JSON so unrelated entries and
fields survive. Do not add shell aliases or edit startup files unless requested.

Verify with `hop list`. Launch `hop NAME` when opening the project is part of
the request; it opens a real application. Report the resulting shortcut and app.
