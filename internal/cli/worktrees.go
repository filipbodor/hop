package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type worktree struct {
	Path, Branch, Repo               string
	Detached, Bare, Prunable, Locked bool
}

func parseWorktrees(data []byte) ([]worktree, error) {
	var result []worktree
	var current *worktree
	for _, raw := range bytes.Split(data, []byte{0}) {
		field := string(raw)
		if field == "" {
			current = nil
			continue
		}
		key, value, _ := strings.Cut(field, " ")
		if key == "worktree" {
			if value == "" || strings.ContainsAny(value, "\r\n") || !filepath.IsAbs(value) {
				return nil, fmt.Errorf("unsupported worktree path %q", value)
			}
			result = append(result, worktree{Path: value})
			current = &result[len(result)-1]
			continue
		}
		if current == nil {
			return nil, fmt.Errorf("invalid git worktree record %q", key)
		}
		switch key {
		case "branch":
			current.Branch = strings.TrimPrefix(value, "refs/heads/")
		case "detached":
			current.Detached = true
		case "bare":
			current.Bare = true
		case "prunable":
			current.Prunable = true
		case "locked":
			current.Locked = true
		}
	}
	return result, nil
}

// Workspace files are read as data. No tasks, scripts, or editor settings run.
func projectFolders(path string) ([]string, error) {
	path, err := expandHome(path)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(filepath.Ext(path), ".code-workspace") {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var workspace struct {
			Folders []struct {
				Path string `json:"path"`
			} `json:"folders"`
		}
		if err := json.Unmarshal(data, &workspace); err != nil {
			return nil, fmt.Errorf("cannot read workspace folders in %s: %w", path, err)
		}
		var folders []string
		for _, folder := range workspace.Folders {
			if folder.Path == "" {
				continue
			}
			value := folder.Path
			if !filepath.IsAbs(value) {
				value = filepath.Join(filepath.Dir(path), value)
			}
			folders = append(folders, value)
		}
		return folders, nil
	}
	return []string{path}, nil
}

func discoverWorktrees(paths []string) ([]worktree, error) {
	seen := map[string]bool{}
	var result []worktree
	for _, path := range paths {
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			continue
		}
		cmd := exec.Command("git", "-C", path, "worktree", "list", "--porcelain", "-z")
		data, err := cmd.Output()
		if err != nil {
			// Non-Git workspace folders are normal; don't hide other Git failures.
			probe := exec.Command("git", "-C", path, "rev-parse", "--git-dir")
			if probe.Run() != nil {
				continue
			}
			return nil, fmt.Errorf("cannot list worktrees in %s: %w", path, err)
		}
		trees, err := parseWorktrees(data)
		if err != nil {
			return nil, err
		}
		repo := ""
		if len(trees) > 0 {
			repo = filepath.Base(trees[0].Path)
		}
		for _, tree := range trees {
			if tree.Bare || seen[tree.Path] {
				continue
			}
			seen[tree.Path] = true
			tree.Repo = repo
			result = append(result, tree)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result, nil
}

func (w worktree) label() string {
	branch := w.Branch
	if w.Detached {
		branch = "detached HEAD"
	}
	status := ""
	if w.Locked {
		status += " [locked]"
	}
	if w.Prunable {
		status += " [prunable]"
	}
	if info, err := os.Stat(w.Path); err != nil || !info.IsDir() {
		status += " [missing]"
	}
	return fmt.Sprintf("%s | %s | %s%s", w.Repo, branch, w.Path, status)
}
