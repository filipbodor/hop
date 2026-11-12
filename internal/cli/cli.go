package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// App keeps streams and the config location injectable for offline tests.
type App struct {
	In          io.Reader
	Out, Err    io.Writer
	ConfigPath  string
	Interactive bool
	Version     string
}

const help = `Hop — your editor workspaces, one command away.

  hop NAME                    Open a registered project
  hop                         Search projects interactively
  hop add NAME [PATH]         Register a folder or saved workspace
  hop add NAME PATH --app APP Choose an app for this project (code, cursor, xcode…)
  hop add NAME PATH --replace Update a saved path
  hop list                    List registered projects
  hop remove NAME             Unregister; keep project files
  hop editor COMMAND [ARGS]   Set editor executable and arguments
  hop config                  Print the private config location
  hop open [NAME]             Explicit form of hop NAME
  hop worktree [WORDS...]     Pick a Git worktree and open editor + Sublime Merge
  hop worktree --list         List worktrees from current/registered repositories
  hop worktree --repo PATH    Limit discovery to one repository
  hop init zsh                Print the hwt helper to also move this terminal
  hop --version               Show version
`

type usageError string

func (e usageError) Error() string { return string(e) }

func (a App) Run(args []string) int {
	code, err := a.run(args)
	if err != nil {
		fmt.Fprintln(a.Err, "hop:", err)
		var usage usageError
		if errors.As(err, &usage) {
			return 2
		}
		return 1
	}
	return code
}

func (a App) run(args []string) (int, error) {
	if len(args) == 1 && args[0] == "--version" {
		fmt.Fprintln(a.Out, "hop", a.Version)
		return 0, nil
	}
	if (len(args) == 1 && isHelp(args[0])) ||
		(len(args) == 2 && commands[args[0]] && isHelp(args[1])) {
		fmt.Fprint(a.Out, help)
		return 0, nil
	}
	command := "open"
	if len(args) > 0 {
		if strings.HasPrefix(args[0], "-") {
			return 0, usageError("unknown option; use hop --help")
		}
		if commands[args[0]] {
			command, args = args[0], args[1:]
		}
	}
	if command == "init" {
		if len(args) != 1 || args[0] != "zsh" {
			return 0, usageError("usage: hop init zsh")
		}
		fmt.Fprint(a.Out, worktreeZsh)
		return 0, nil
	}
	path := a.ConfigPath
	if path == "" {
		var err error
		path, err = configPath()
		if err != nil {
			return 0, err
		}
	}
	if command == "config" {
		if len(args) != 0 {
			return 0, usageError("usage: hop config")
		}
		fmt.Fprintln(a.Out, path)
		return 0, nil
	}
	c, err := loadConfig(path)
	if err != nil {
		return 0, err
	}
	switch command {
	case "worktree":
		return a.openWorktree(c, args)
	case "add":
		replace, positional, options := false, []string{}, true
		app, appSet := "", false
		for i := 0; i < len(args); i++ {
			arg := args[i]
			switch {
			case options && arg == "--":
				options = false
			case options && arg == "--replace":
				replace = true
			case options && (arg == "--app" || strings.HasPrefix(arg, "--app=")):
				if arg == "--app" {
					if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
						return 0, usageError("--app requires an app, for example: --app xcode")
					}
					i++
					app = args[i]
				} else {
					app = strings.TrimPrefix(arg, "--app=")
				}
				if strings.TrimSpace(app) == "" {
					return 0, usageError("--app must not be empty; use --app default to reset it")
				}
				appSet = true
			case options && strings.HasPrefix(arg, "-"):
				return 0, usageError("unknown add option: " + arg)
			default:
				positional = append(positional, arg)
			}
		}
		if len(positional) < 1 || len(positional) > 2 {
			return 0, usageError("usage: hop add NAME [PATH] [--app APP] [--replace]")
		}
		name, target := positional[0], "."
		if len(positional) == 2 {
			target = positional[1]
		}
		if err := validateName(name); err != nil {
			return 0, err
		}
		target, err = targetPath(target)
		if err != nil {
			return 0, err
		}
		registered, exists := c.Projects[name]
		if exists && !replace {
			return 0, fmt.Errorf("project %q already exists; use --replace to update it", name)
		}
		registered.Path = target
		if appSet {
			registered.App = app
			if app == "default" {
				registered.App = ""
			}
		}
		c.Projects[name] = registered
		if err := saveConfig(path, c); err != nil {
			return 0, err
		}
		fmt.Fprintf(a.Out, "Registered %s: %s%s\n", name, target, appLabel(registered))
	case "remove":
		if len(args) != 1 {
			return 0, usageError("usage: hop remove NAME")
		}
		if _, exists := c.Projects[args[0]]; !exists {
			return 0, fmt.Errorf("unknown project %q", args[0])
		}
		delete(c.Projects, args[0])
		if err := saveConfig(path, c); err != nil {
			return 0, err
		}
		fmt.Fprintf(a.Out, "Unregistered %s; files kept.\n", args[0])
	case "list":
		if len(args) != 0 {
			return 0, usageError("usage: hop list")
		}
		if len(c.Projects) == 0 {
			fmt.Fprintln(a.Out, "No projects yet. Register one with: hop add NAME [PATH]")
		}
		for _, name := range sortedNames(c.Projects) {
			registered := c.Projects[name]
			value := registered.Path
			expanded, err := expandHome(value)
			missing := ""
			if _, statErr := os.Stat(expanded); err != nil || statErr != nil {
				missing = " [missing]"
			}
			fmt.Fprintf(a.Out, "%s\t%s%s%s\n", name, value, appLabel(registered), missing)
		}
	case "editor":
		if len(args) == 0 {
			return 0, usageError("specify an editor, for example: hop editor code --new-window")
		}
		for _, arg := range args {
			if arg == "" {
				return 0, usageError("editor arguments must not be empty")
			}
		}
		c.Editor = args
		if err := saveConfig(path, c); err != nil {
			return 0, err
		}
		encoded, _ := json.Marshal(args)
		fmt.Fprintf(a.Out, "Editor: %s\n", encoded)
	case "open":
		if len(args) > 1 || (len(args) == 1 && strings.HasPrefix(args[0], "-")) {
			return 0, usageError("usage: hop [open] NAME")
		}
		name := ""
		if len(args) == 1 {
			name = args[0]
		} else {
			var err error
			name, err = a.choose(c.Projects)
			if errors.Is(err, io.EOF) {
				return 130, nil
			}
			if err != nil {
				return 0, err
			}
		}
		if name == "" {
			return 0, nil
		}
		value, exists := c.Projects[name]
		if !exists {
			return 0, fmt.Errorf("unknown project %q.%s", name, suggest(name, c.Projects))
		}
		target, err := targetPath(value.Path)
		if err != nil {
			return 0, err
		}
		editor, err := editorFor(value, c.Editor, runtime.GOOS)
		if err != nil {
			return 0, err
		}
		cmd := exec.Command(editor[0], append(editor[1:], target)...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = a.In, a.Out, a.Err
		if err := cmd.Run(); err != nil {
			var exited *exec.ExitError
			if errors.As(err, &exited) {
				if exited.ExitCode() < 0 {
					return 130, nil
				}
				return exited.ExitCode(), nil
			}
			if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
				return 0, fmt.Errorf("editor %q not found; install its CLI or update the project's --app or hop editor COMMAND", editor[0])
			}
			return 0, err
		}
	}
	return 0, nil
}

func isHelp(arg string) bool { return arg == "--help" || arg == "-h" }

func sortedNames(projects map[string]project) []string {
	names := make([]string, 0, len(projects))
	for name := range projects {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (a App) choose(projects map[string]project) (string, error) {
	if len(projects) == 0 {
		return "", errors.New("no projects yet; register one with: hop add NAME [PATH]")
	}
	if !a.Interactive {
		return "", errors.New("choose a project with hop NAME, or run hop in an interactive terminal")
	}
	names := sortedNames(projects)
	matches := names
	scanner := bufio.NewScanner(a.In)
	for {
		for i, name := range matches {
			fmt.Fprintf(a.Out, "  %d. %s  %s%s\n", i+1, name, projects[name].Path, appLabel(projects[name]))
		}
		fmt.Fprint(a.Out, "Open a name or number, type to filter, or Enter to cancel: ")
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return "", err
			}
			return "", io.EOF
		}
		answer := strings.TrimSpace(scanner.Text())
		if answer == "" {
			return "", nil
		}
		if _, exists := projects[answer]; exists {
			return answer, nil
		}
		if i, err := strconv.Atoi(answer); err == nil && i > 0 && i <= len(matches) {
			return matches[i-1], nil
		}
		matches = nil
		for _, name := range names {
			if strings.Contains(strings.ToLower(name), strings.ToLower(answer)) {
				matches = append(matches, name)
			}
		}
		if len(matches) == 0 {
			fmt.Fprintln(a.Out, "No matches. Try another name.")
			matches = names
		}
	}
}

func suggest(name string, projects map[string]project) string {
	for _, candidate := range sortedNames(projects) {
		if editDistance(strings.ToLower(name), strings.ToLower(candidate)) <= max(1, len(candidate)/3) {
			return fmt.Sprintf(" Did you mean %s?", candidate)
		}
	}
	return " Use hop list."
}

func appLabel(p project) string {
	if p.App != "" {
		return " [" + p.App + "]"
	}
	return ""
}

func editorFor(p project, fallback []string, goos string) ([]string, error) {
	if p.App == "" || p.App == "default" {
		return append([]string(nil), fallback...), nil
	}
	app := p.App
	if strings.EqualFold(app, "xcode") || strings.HasSuffix(strings.ToLower(app), ".app") {
		if goos != "darwin" {
			return nil, fmt.Errorf("app %q requires macOS", app)
		}
		if strings.EqualFold(app, "xcode") {
			app = "Xcode"
		}
		return []string{"/usr/bin/open", "-a", app}, nil
	}
	return []string{app}, nil
}

func editDistance(a, b string) int {
	previous := make([]int, len(b)+1)
	for i := range previous {
		previous[i] = i
	}
	for i := range a {
		current := make([]int, len(b)+1)
		current[0] = i + 1
		for j := range b {
			cost := 0
			if a[i] != b[j] {
				cost = 1
			}
			current[j+1] = min(previous[j+1]+1, current[j]+1, previous[j]+cost)
		}
		previous = current
	}
	return previous[len(b)]
}
