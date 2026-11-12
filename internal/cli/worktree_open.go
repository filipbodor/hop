package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const worktreeZsh = `# Hop worktree picker: open apps and move this terminal to the same checkout.
function hwt() {
    local destination argument
    for argument in "$@"; do
        [[ "$argument" != "--" ]] || break
        if [[ "$argument" == "--list" || "$argument" == "--help" || "$argument" == "-h" ]]; then
            command hop worktree "$@"
            return $?
        fi
    done
    destination=$(command hop worktree "$@") || return $?
    [[ -n "$destination" ]] || return 0
    builtin cd -- "$destination"
}
`

func (a App) openWorktree(c config, args []string) (int, error) {
	list, pathOnly, options := false, false, true
	repo := ""
	var words []string
	for i := 0; i < len(args); i++ {
		switch {
		case options && args[i] == "--":
			options = false
		case options && args[i] == "--list":
			list = true
		case options && args[i] == "--path-only":
			pathOnly = true
		case options && args[i] == "--repo":
			if i+1 >= len(args) {
				return 0, usageError("--repo requires a repository directory")
			}
			i++
			repo = args[i]
		case options && strings.HasPrefix(args[i], "-"):
			return 0, usageError("usage: hop worktree [--repo PATH] [--list | --path-only] [WORDS...]")
		default:
			words = append(words, strings.ToLower(args[i]))
		}
	}
	if list && pathOnly {
		return 0, usageError("choose --list or --path-only")
	}
	var folders []string
	if repo != "" {
		path, err := expandHome(repo)
		if err != nil {
			return 0, err
		}
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			return 0, fmt.Errorf("repository directory unavailable: %s", path)
		}
		folders = append(folders, path)
	} else {
		if cwd, err := os.Getwd(); err == nil {
			folders = append(folders, cwd)
		}
		for _, name := range sortedNames(c.Projects) {
			paths, err := projectFolders(c.Projects[name].Path)
			if err != nil {
				fmt.Fprintf(a.Err, "hop: skipping %s: %v\n", name, err)
				continue
			}
			folders = append(folders, paths...)
		}
	}
	trees, err := discoverWorktrees(folders)
	if err != nil {
		return 0, err
	}
	candidates := map[string]project{}
	for _, tree := range trees {
		label := tree.label()
		matches := true
		for _, word := range words {
			if !strings.Contains(strings.ToLower(label), word) {
				matches = false
				break
			}
		}
		if matches {
			candidates[label] = project{Path: tree.Path}
		}
	}
	if len(candidates) == 0 {
		return 0, errors.New("no matching worktrees; register a repository/workspace with hop add, run inside a repository, or use --repo PATH")
	}
	if list {
		for _, name := range sortedNames(candidates) {
			fmt.Fprintln(a.Out, name)
		}
		return 0, nil
	}
	selected := ""
	if len(candidates) == 1 {
		selected = sortedNames(candidates)[0]
	} else {
		if !a.Interactive {
			return 0, errors.New("multiple matching worktrees; use more search words, --list, or run hwt in an interactive terminal")
		}
		picker := a
		picker.Out = a.Err // stdout is reserved for the selected path, including under $(...).
		display := map[string]project{}
		for name := range candidates {
			display[name] = project{}
		}
		selected, err = picker.choose(display)
		if errors.Is(err, io.EOF) {
			return 130, nil
		}
		if err != nil {
			return 0, fmt.Errorf("%w; narrow the worktree query or use --list", err)
		}
		if selected == "" {
			return 0, nil
		}
	}
	target, err := targetPath(candidates[selected].Path)
	if err != nil {
		return 0, err
	}
	if !pathOnly {
		if err := a.launchWorktree(c.Editor, target); err != nil {
			return 0, err
		}
	}
	fmt.Fprintln(a.Out, target)
	return 0, nil
}

func mergeExecutable() (string, error) {
	if path, err := exec.LookPath("smerge"); err == nil {
		return path, nil
	}
	if runtime.GOOS == "darwin" {
		home, _ := os.UserHomeDir()
		for _, base := range []string{"/Applications", filepath.Join(home, "Applications")} {
			path := filepath.Join(base, "Sublime Merge.app", "Contents", "SharedSupport", "bin", "smerge")
			if found, err := exec.LookPath(path); err == nil {
				return found, nil
			}
		}
	}
	return "", errors.New("Sublime Merge not found; install its smerge CLI or macOS application")
}

func (a App) launchWorktree(editor []string, target string) error {
	if len(editor) == 0 {
		return errors.New("configure an editor with hop editor COMMAND")
	}
	if _, err := exec.LookPath(editor[0]); err != nil {
		return fmt.Errorf("editor %q not found: %w", editor[0], err)
	}
	merge, err := mergeExecutable()
	if err != nil {
		return err
	}
	command := append([]string(nil), editor...)
	if base := filepath.Base(command[0]); base == "code" || base == "code-insiders" || base == "cursor" {
		command = append(command, "--new-window")
	}
	for _, args := range [][]string{append(command, target), {merge, target}} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = target // GUI CLIs also work when the caller's original cwd was removed.
		cmd.Stdin, cmd.Stdout, cmd.Stderr = a.In, a.Err, a.Err
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("opening %s failed: %w", args[0], err)
		}
	}
	return nil
}
