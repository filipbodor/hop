# Changelog

## 0.3.0

- Choose an app per project with `hop add NAME PATH --app APP`.
- Support Xcode and other macOS `.app` applications alongside editor CLIs.
- Show app overrides in the project list and picker; preserve existing path-only entries.
- Keep the app when updating a saved path, or reset it with `--app default`.

## 0.2.0

- Rename dev-workspaces to Hop, with `hop` as the command.
- Replace Python with Go and provide standalone macOS/Linux binaries for ARM64 and AMD64.
- Preserve existing projects and editor arguments through the legacy config fallback.
- Add checksums and automated releases for version tags.

## 0.1.0

- Initial Python release: named folders and VS Code workspaces, interactive picker,
  private configuration, and editor selection.
