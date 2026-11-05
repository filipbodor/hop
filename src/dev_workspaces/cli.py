"""A local registry of editor workspaces. No shell commands or network calls."""

import argparse
import difflib
import json
import os
import re
import subprocess
import sys
import tempfile
from pathlib import Path

from . import __version__

RESERVED = {"add", "remove", "list", "editor", "config", "open"}


class UserError(Exception):
    pass


def config_path():
    root = Path(os.environ.get("XDG_CONFIG_HOME") or Path.home() / ".config")
    return root / "dev-workspaces" / "config.json"


def load_config(path):
    if not path.exists():
        return {"editor": ["code"], "projects": {}}
    try:
        data = json.loads(path.read_text())
    except (OSError, ValueError) as exc:
        raise UserError(f"Cannot read config at {path}: {exc}") from exc
    if not isinstance(data, dict):
        raise UserError(f"Config at {path} must be a JSON object.")
    editor = data.get("editor", ["code"])
    projects = data.get("projects", {})
    if (
        not isinstance(editor, list)
        or not editor
        or any(not isinstance(arg, str) or not arg for arg in editor)
        or not isinstance(projects, dict)
        or any(not isinstance(p, str) or not p for p in projects.values())
    ):
        raise UserError("Config needs an editor argument list and a projects name-to-path object.")
    for name in projects:
        validate_name(name)
    return {**data, "editor": editor, "projects": projects}


def save_config(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    # Replace atomically so a failed write cannot truncate the registry.
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(mode="w", dir=path.parent, delete=False) as stream:
            temporary = Path(stream.name)
            json.dump(data, stream, indent=2, sort_keys=True)
            stream.write("\n")
        temporary.replace(path)
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


def validate_name(name):
    if not re.fullmatch(r"[a-zA-Z0-9][a-zA-Z0-9_-]*", name) or name in RESERVED:
        raise UserError(
            f"Invalid project name {name!r}. Use letters, numbers, hyphens or underscores; "
            "command names are reserved."
        )


def target_path(value):
    path = Path(value).expanduser().resolve()
    if not path.is_dir() and not (path.is_file() and path.suffix == ".code-workspace"):
        raise UserError(f"Not a folder or .code-workspace file: {path}")
    return path


def choose_project(projects):
    if not projects:
        raise UserError("No projects yet. Register one with: dev add NAME [PATH]")
    if not sys.stdin.isatty():
        raise UserError("Choose a project with dev NAME, or run dev in an interactive terminal.")
    matches = sorted(projects)
    while True:
        for number, name in enumerate(matches, 1):
            print(f"  {number}. {name}  {projects[name]}")
        answer = input("Open a name or number, type to filter, or Enter to cancel: ").strip()
        if not answer:
            return None
        if answer in projects:
            return answer
        if answer.isdigit() and 1 <= int(answer) <= len(matches):
            return matches[int(answer) - 1]
        matches = [name for name in sorted(projects) if answer.casefold() in name.casefold()]
        if not matches:
            print("No matches. Try another name.")
            matches = sorted(projects)


def open_project(name, config):
    projects = config["projects"]
    if name not in projects:
        suggestions = difflib.get_close_matches(name, projects, n=3)
        hint = f" Did you mean {', '.join(suggestions)}?" if suggestions else " Use dev list."
        raise UserError(f"Unknown project {name!r}.{hint}")
    path = target_path(projects[name])
    try:
        # A list preserves spaces and metacharacters without invoking a shell.
        return subprocess.run([*config["editor"], str(path)], check=False).returncode
    except FileNotFoundError as exc:
        raise UserError(
            f"Editor {config['editor'][0]!r} not found. Install its CLI or use dev editor COMMAND."
        ) from exc


def parser():
    result = argparse.ArgumentParser(
        prog="dev", description="Open editor workspaces by name. Run dev alone for a picker."
    )
    result.add_argument("--version", action="version", version=f"dev-workspaces {__version__}")
    commands = result.add_subparsers(dest="command")
    add = commands.add_parser("add", help="Register a folder or .code-workspace file")
    add.add_argument("name")
    add.add_argument("path", nargs="?", default=".")
    add.add_argument("--replace", action="store_true", help="Update an existing name")
    remove = commands.add_parser("remove", help="Unregister a project; keep its files")
    remove.add_argument("name")
    commands.add_parser("list", help="List registered projects")
    commands.add_parser("config", help="Print the private configuration file location")
    editor = commands.add_parser("editor", help="Set editor executable and optional arguments")
    editor.add_argument("arguments", nargs=argparse.REMAINDER)
    opened = commands.add_parser("open", help="Open a registered project")
    opened.add_argument("name", nargs="?")
    return result


def run(argv):
    # Keep management commands explicit while allowing the short form: dev NAME.
    if argv and argv[0] not in RESERVED and not argv[0].startswith("-"):
        argv = ["open", *argv]
    args = parser().parse_args(argv)
    path = config_path()
    if args.command == "config":
        print(path)
        return 0
    config = load_config(path)
    projects = config["projects"]
    if args.command == "add":
        validate_name(args.name)
        target = target_path(args.path)
        if args.name in projects and not args.replace:
            raise UserError(f"Project {args.name!r} already exists. Use --replace to update it.")
        projects[args.name] = str(target)
        save_config(path, config)
        print(f"Registered {args.name}: {target}")
    elif args.command == "remove":
        if args.name not in projects:
            raise UserError(f"Unknown project {args.name!r}.")
        del projects[args.name]
        save_config(path, config)
        print(f"Unregistered {args.name}; files kept.")
    elif args.command == "list":
        if not projects:
            print("No projects yet. Register one with: dev add NAME [PATH]")
        for name, value in sorted(projects.items()):
            missing = " [missing]" if not Path(value).expanduser().exists() else ""
            print(f"{name}\t{value}{missing}")
    elif args.command == "editor":
        if not args.arguments:
            raise UserError("Specify an editor, for example: dev editor code --new-window")
        config["editor"] = args.arguments
        save_config(path, config)
        print(f"Editor: {json.dumps(args.arguments)}")
    else:
        name = args.name if args.command == "open" else None
        if name is None:
            name = choose_project(projects)
        if name is not None:
            return open_project(name, config)
    return 0


def main(argv=None):
    try:
        return run(list(sys.argv[1:] if argv is None else argv))
    except (UserError, OSError) as exc:
        print(f"dev: {exc}", file=sys.stderr)
        return 1
    except (KeyboardInterrupt, EOFError):
        return 130
