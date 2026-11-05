# dev-workspaces

- Keep Python 3.11 compatibility and runtime dependencies empty.
- User paths and editor preferences belong in private config, never in this repository.
- Launch editors with argument lists, never shell evaluation. Do not execute workspace contents.
- Preserve registered projects and require `--replace` before overwriting a name.
- Run `python -m unittest discover -s tests -v`, `ruff check .`, and `ruff format --check .`.
- Build and smoke-test the wheel when changing packaging or entry points.
