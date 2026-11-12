package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type config struct {
	Editor   []string
	Projects map[string]project
	extra    map[string]json.RawMessage
}

// A plain path remains valid for projects using the default editor.
type project struct {
	Path  string
	App   string
	extra map[string]json.RawMessage
}

func (p *project) UnmarshalJSON(data []byte) error {
	*p = project{}
	if err := json.Unmarshal(data, &p.Path); err == nil && string(data) != "null" {
		return nil
	}
	if err := json.Unmarshal(data, &p.extra); err != nil || p.extra == nil {
		return errors.New("project must be a path string or an object with path and optional app")
	}
	if err := json.Unmarshal(p.extra["path"], &p.Path); err != nil {
		return errors.New("project path must be a nonempty string")
	}
	if value, ok := p.extra["app"]; ok {
		if err := json.Unmarshal(value, &p.App); err != nil || p.App == "" {
			return errors.New("project app must be a nonempty string")
		}
	}
	return nil
}

func (p project) MarshalJSON() ([]byte, error) {
	if p.App == "" && len(p.extra) == 0 {
		return json.Marshal(p.Path)
	}
	data := make(map[string]json.RawMessage, len(p.extra)+2)
	for key, value := range p.extra {
		data[key] = value
	}
	data["path"], _ = json.Marshal(p.Path)
	if p.App != "" {
		data["app"], _ = json.Marshal(p.App)
	} else {
		delete(data, "app")
	}
	return json.Marshal(data)
}

func configPath() (string, error) {
	root := os.Getenv("XDG_CONFIG_HOME")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		root = filepath.Join(home, ".config")
	}
	path := filepath.Join(root, "hop", "config.json")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		// Existing installations keep their registry; never silently reset it.
		legacy := filepath.Join(root, "dev-workspaces", "config.json")
		if _, err := os.Stat(legacy); !errors.Is(err, os.ErrNotExist) {
			return legacy, nil
		}
	}
	return path, nil
}

func loadConfig(path string) (config, error) {
	c := config{Editor: []string{"code"}, Projects: map[string]project{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, fmt.Errorf("cannot read config at %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &c.extra); err != nil || c.extra == nil {
		return c, fmt.Errorf("config at %s must be a JSON object", path)
	}
	if value, exists := c.extra["editor"]; exists {
		c.Editor = nil
		if err := json.Unmarshal(value, &c.Editor); err != nil || len(c.Editor) == 0 {
			return c, errors.New("config editor must be a nonempty argument list")
		}
	}
	for _, arg := range c.Editor {
		if arg == "" {
			return c, errors.New("config editor arguments must not be empty")
		}
	}
	if value, exists := c.extra["projects"]; exists {
		if err := json.Unmarshal(value, &c.Projects); err != nil || c.Projects == nil {
			return c, errors.New("config projects must map names to paths or objects with path and optional app")
		}
	}
	for name, project := range c.Projects {
		// These names became commands later; existing registries still open them
		// explicitly with `hop open NAME`, while new registrations reserve them.
		if err := validateName(name); err != nil && name != "worktree" && name != "init" {
			return c, err
		}
		if project.Path == "" {
			return c, fmt.Errorf("project %q has an empty path", name)
		}
	}
	return c, nil
}

func saveConfig(path string, c config) error {
	data := c.extra
	if data == nil {
		data = map[string]json.RawMessage{}
	}
	data["editor"], _ = json.Marshal(c.Editor)
	data["projects"], _ = json.Marshal(c.Projects)
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".hop-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	// CreateTemp uses mode 0600. Rename only after a successful write and close.
	if _, err := f.Write(append(encoded, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)
var commands = map[string]bool{
	"add": true, "remove": true, "list": true, "editor": true, "config": true, "open": true,
	"worktree": true, "init": true,
}

func validateName(name string) error {
	if !namePattern.MatchString(name) || commands[name] {
		return fmt.Errorf("invalid project name %q: use letters, numbers, hyphens or underscores; command names are reserved", name)
	}
	return nil
}

func expandHome(value string) (string, error) {
	if value == "~" || strings.HasPrefix(value, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if value == "~" {
			return home, nil
		}
		return filepath.Join(home, strings.TrimPrefix(value, "~/")), nil
	}
	return value, nil
}

func targetPath(value string) (string, error) {
	expanded, err := expandHome(value)
	if err != nil {
		return "", err
	}
	path, err := filepath.Abs(expanded)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil || (!info.IsDir() && !(info.Mode().IsRegular() && filepath.Ext(path) == ".code-workspace")) {
		return "", fmt.Errorf("not a folder or .code-workspace file: %s", path)
	}
	return filepath.EvalSymlinks(path)
}
