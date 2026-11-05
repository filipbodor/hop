package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type harness struct {
	app                     App
	out, stderr             bytes.Buffer
	root, folder, workspace string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{root: root, folder: filepath.Join(root, "project with spaces; $(nothing)"), workspace: filepath.Join(root, "web.code-workspace")}
	if err := os.Mkdir(h.folder, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(h.workspace, []byte(`{"folders":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	h.app = App{In: strings.NewReader(""), Out: &h.out, Err: &h.stderr, ConfigPath: filepath.Join(root, "config", "config.json"), Version: "test"}
	return h
}

func (h *harness) run(args ...string) int {
	h.out.Reset()
	h.stderr.Reset()
	return h.app.Run(args)
}

func (h *harness) want(t *testing.T, code int, args ...string) {
	t.Helper()
	if got := h.run(args...); got != code {
		t.Fatalf("%v: exit %d, want %d; %s", args, got, code, h.stderr.String())
	}
}

func (h *harness) config(t *testing.T) config {
	t.Helper()
	c, err := loadConfig(h.app.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestRegisterAndList(t *testing.T) {
	h := newHarness(t)
	h.want(t, 0, "add", "web", h.workspace)
	h.want(t, 0, "add", "calendar", h.folder)
	h.want(t, 0, "list")
	if !strings.Contains(h.out.String(), "web\t"+h.workspace) || !strings.HasPrefix(h.out.String(), "calendar\t") {
		t.Fatal(h.out.String())
	}
	if h.config(t).Projects["web"] != h.workspace {
		t.Fatal("workspace path changed")
	}
}

func TestDefaultAndRelativePath(t *testing.T) {
	h := newHarness(t)
	h.want(t, 0, "add", "current")
	want, err := targetPath(".")
	if err != nil {
		t.Fatal(err)
	}
	if h.config(t).Projects["current"] != want {
		t.Fatal("default path is not cwd")
	}
	relative, err := filepath.Rel(want, h.folder)
	if err != nil {
		t.Fatal(err)
	}
	h.want(t, 0, "add", "relative", relative)
	if h.config(t).Projects["relative"] != h.folder {
		t.Fatal("relative path resolved incorrectly")
	}
}

func TestHomeExpansion(t *testing.T) {
	h := newHarness(t)
	t.Setenv("HOME", h.root)
	for _, value := range []string{"~", "~/"} {
		got, err := targetPath(value)
		if err != nil || got != h.root {
			t.Fatalf("%s: %s %v", value, got, err)
		}
	}
	h.want(t, 0, "add", "web", "~/web.code-workspace")
	if h.config(t).Projects["web"] != h.workspace {
		t.Fatal("home path resolved incorrectly")
	}
}

func TestReplaceRequired(t *testing.T) {
	h := newHarness(t)
	h.want(t, 0, "add", "web", h.folder)
	before, _ := os.ReadFile(h.app.ConfigPath)
	h.want(t, 1, "add", "web", h.workspace)
	after, _ := os.ReadFile(h.app.ConfigPath)
	if !bytes.Equal(before, after) {
		t.Fatal("duplicate changed config")
	}
	h.want(t, 0, "add", "web", h.workspace, "--replace")
	if h.config(t).Projects["web"] != h.workspace {
		t.Fatal("replacement failed")
	}
}

func TestInvalidNames(t *testing.T) {
	for _, name := range []string{"add", "open", "two words", "../bad", "-option", "", "café"} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.want(t, 1, "add", "--", name, h.folder)
		})
	}
}

func TestRejectInvalidTargets(t *testing.T) {
	h := newHarness(t)
	file := filepath.Join(h.root, "file.txt")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{file, filepath.Join(h.root, "missing")} {
		h.want(t, 1, "add", "web", target)
	}
	if _, err := os.Stat(h.app.ConfigPath); !os.IsNotExist(err) {
		t.Fatal("invalid target created config")
	}
}

func TestRemoveKeepsFiles(t *testing.T) {
	h := newHarness(t)
	h.want(t, 0, "add", "web", h.folder)
	h.want(t, 0, "remove", "web")
	if _, err := os.Stat(h.folder); err != nil {
		t.Fatal(err)
	}
	if len(h.config(t).Projects) != 0 {
		t.Fatal("project not removed")
	}
	h.want(t, 1, "remove", "web")
}

func TestEditorProcess(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprint(explicit), func(t *testing.T) {
			h := newHarness(t)
			h.want(t, 0, "add", "web", h.folder)
			capture := filepath.Join(h.root, "captured.json")
			t.Setenv("HOP_TEST_EDITOR", "1")
			t.Setenv("HOP_TEST_CAPTURE", capture)
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			h.want(t, 0, "editor", executable, "-test.run=^TestHelperProcess$", "--", "--new-window", "literal; $(touch nope)")
			if explicit {
				h.want(t, 7, "open", "web")
			} else {
				h.want(t, 7, "web")
			}
			data, err := os.ReadFile(capture)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			want := []string{"--new-window", "literal; $(touch nope)", h.folder}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %q, want %q", got, want)
			}
			if !strings.Contains(h.out.String(), "editor stdout") || !strings.Contains(h.stderr.String(), "editor stderr") {
				t.Fatal("editor output not forwarded")
			}
		})
	}
}

// Re-execute the Go test binary as a fake editor: no IDE, shell or Python needed.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("HOP_TEST_EDITOR") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			data, err := json.Marshal(os.Args[i+1:])
			if err != nil {
				os.Exit(99)
			}
			if err := os.WriteFile(os.Getenv("HOP_TEST_CAPTURE"), data, 0600); err != nil {
				os.Exit(99)
			}
			fmt.Fprintln(os.Stdout, "editor stdout")
			fmt.Fprintln(os.Stderr, "editor stderr")
			os.Exit(7)
		}
	}
	os.Exit(99)
}

func TestMissingEditorAndTarget(t *testing.T) {
	h := newHarness(t)
	h.want(t, 0, "add", "web", h.folder)
	h.want(t, 0, "editor", filepath.Join(h.root, "missing-editor"))
	h.want(t, 1, "web")
	if !strings.Contains(h.stderr.String(), "hop editor") {
		t.Fatal(h.stderr.String())
	}
	if err := os.Remove(h.folder); err != nil {
		t.Fatal(err)
	}
	h.want(t, 1, "web")
	h.want(t, 0, "list")
	if !strings.Contains(h.out.String(), "[missing]") {
		t.Fatal(h.out.String())
	}
}

func TestUnknownNameSuggestion(t *testing.T) {
	h := newHarness(t)
	h.want(t, 0, "add", "web", h.folder)
	h.want(t, 1, "webb")
	if !strings.Contains(h.stderr.String(), "Did you mean web?") {
		t.Fatal(h.stderr.String())
	}
}

func TestUsageAndHelp(t *testing.T) {
	h := newHarness(t)
	for _, args := range [][]string{{"add"}, {"remove"}, {"list", "extra"}, {"config", "extra"}, {"editor"}, {"open", "one", "two"}, {"--unknown"}, {"add", "x", "--unknown"}} {
		h.want(t, 2, args...)
	}
	for _, args := range [][]string{{"--help"}, {"-h"}, {"add", "--help"}, {"--version"}} {
		h.want(t, 0, args...)
		if !strings.Contains(strings.ToLower(h.out.String()), "hop") {
			t.Fatal(h.out.String())
		}
	}
}

func TestConfigLocationAndEmptyList(t *testing.T) {
	h := newHarness(t)
	h.want(t, 0, "config")
	if strings.TrimSpace(h.out.String()) != h.app.ConfigPath {
		t.Fatal(h.out.String())
	}
	h.want(t, 0, "list")
	if !strings.Contains(h.out.String(), "hop add") {
		t.Fatal(h.out.String())
	}
	if _, err := os.Stat(h.app.ConfigPath); !os.IsNotExist(err) {
		t.Fatal("read-only commands created config")
	}
}

func TestPicker(t *testing.T) {
	projects := map[string]string{"calendar": "/example/calendar", "web": "/example/web"}
	for _, tc := range []struct {
		input, want string
		eof         bool
	}{
		{"cal\n1\n", "calendar", false}, {"web\n", "web", false}, {"2\n", "web", false},
		{"\n", "", false}, {"", "", true}, {"nothing\nWEB\n1\n", "web", false},
	} {
		t.Run(fmt.Sprintf("%q", tc.input), func(t *testing.T) {
			h := newHarness(t)
			h.app.Interactive, h.app.In = true, strings.NewReader(tc.input)
			got, err := h.app.choose(projects)
			if got != tc.want || (err != nil) != tc.eof {
				t.Fatalf("got %q, %v", got, err)
			}
		})
	}
}

func TestPickerErrorsAndCancel(t *testing.T) {
	h := newHarness(t)
	h.want(t, 1)
	h.want(t, 0, "add", "web", h.folder)
	h.want(t, 1)
	if !strings.Contains(h.stderr.String(), "interactive terminal") {
		t.Fatal(h.stderr.String())
	}
	h.app.Interactive = true
	h.app.In = strings.NewReader("\n")
	h.want(t, 0)
	h.app.In = strings.NewReader("")
	h.want(t, 130, "open")
}

func TestMalformedConfigPreserved(t *testing.T) {
	for _, content := range []string{`{"bad":`, `[]`, `null`, `{"editor":"code"}`, `{"editor":[]}`, `{"editor":[null]}`, `{"projects":{"x":3}}`, `{"projects":null}`, `{"projects":{"x":""}}`} {
		t.Run(content, func(t *testing.T) {
			h := newHarness(t)
			if err := os.MkdirAll(filepath.Dir(h.app.ConfigPath), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(h.app.ConfigPath, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			h.want(t, 1, "add", "web", h.folder)
			after, _ := os.ReadFile(h.app.ConfigPath)
			if string(after) != content {
				t.Fatal("invalid config overwritten")
			}
		})
	}
}

func TestUnknownFieldsAndPrivatePermissions(t *testing.T) {
	h := newHarness(t)
	if err := os.MkdirAll(filepath.Dir(h.app.ConfigPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(h.app.ConfigPath, []byte(`{"future":{"keep":true}}`), 0600); err != nil {
		t.Fatal(err)
	}
	h.want(t, 0, "add", "web", h.folder)
	var future map[string]bool
	if err := json.Unmarshal(h.config(t).extra["future"], &future); err != nil || !future["keep"] {
		t.Fatal("unknown config field changed")
	}
	info, err := os.Stat(h.app.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatal("config not private")
	}
}

func TestFailedSaveCleansTemporaryFile(t *testing.T) {
	h := newHarness(t)
	// Renaming a regular file onto a directory must fail after writing the temp file.
	if err := os.MkdirAll(h.app.ConfigPath, 0700); err != nil {
		t.Fatal(err)
	}
	if err := saveConfig(h.app.ConfigPath, config{Editor: []string{"code"}, Projects: map[string]string{}}); err == nil {
		t.Fatal("expected rename error")
	}
	entries, err := os.ReadDir(filepath.Dir(h.app.ConfigPath))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		t.Fatal("temporary file leaked")
	}
}

func TestLegacyConfigAndNewConfigPrecedence(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	newPath := filepath.Join(root, "hop", "config.json")
	legacy := filepath.Join(root, "dev-workspaces", "config.json")
	got, err := configPath()
	if err != nil || got != newPath {
		t.Fatalf("got %s %v", got, err)
	}
	c := config{Editor: []string{"code", "--new-window"}, Projects: map[string]string{"web": "/example/web"}}
	if err := saveConfig(legacy, c); err != nil {
		t.Fatal(err)
	}
	got, err = configPath()
	if err != nil || got != legacy {
		t.Fatalf("got %s %v", got, err)
	}
	loaded, err := loadConfig(got)
	if err != nil || !reflect.DeepEqual(loaded.Editor, c.Editor) || !reflect.DeepEqual(loaded.Projects, c.Projects) {
		t.Fatal("legacy registry changed")
	}
	if err := saveConfig(newPath, c); err != nil {
		t.Fatal(err)
	}
	got, err = configPath()
	if err != nil || got != newPath {
		t.Fatalf("got %s %v", got, err)
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", root)
	got, err = configPath()
	if err != nil || got != filepath.Join(root, ".config", "hop", "config.json") {
		t.Fatalf("got %s %v", got, err)
	}
}
