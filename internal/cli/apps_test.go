package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Act as an editor executable without requiring an installed IDE in CI.
func TestMain(m *testing.M) {
	if os.Getenv("HOP_TEST_APP_HELPER") == "1" {
		data, err := json.Marshal(os.Args[1:])
		if err != nil {
			os.Exit(99)
		}
		if err := os.WriteFile(os.Getenv("HOP_TEST_CAPTURE"), data, 0600); err != nil {
			os.Exit(99)
		}
		os.Exit(7)
	}
	os.Exit(m.Run())
}

func TestPerProjectAppOverridesDefaultEditor(t *testing.T) {
	h := newHarness(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	capture := filepath.Join(h.root, "args.json")
	t.Setenv("HOP_TEST_APP_HELPER", "1")
	t.Setenv("HOP_TEST_CAPTURE", capture)
	h.want(t, 0, "editor", filepath.Join(h.root, "missing-global-editor"), "--global-flag")
	h.want(t, 0, "add", "web", h.workspace, "--app", executable)
	h.want(t, 7, "web")
	data, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	var args []string
	if err := json.Unmarshal(data, &args); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(args, []string{h.workspace}) {
		t.Fatalf("editor received %q", args)
	}
	if got := h.config(t).Editor; !reflect.DeepEqual(got, []string{filepath.Join(h.root, "missing-global-editor"), "--global-flag"}) {
		t.Fatal("default editor changed")
	}
}

func TestAppSelection(t *testing.T) {
	fallback := []string{"code", "--new-window"}
	for _, tc := range []struct {
		app, goos string
		want      []string
		wantError bool
	}{
		{"", "linux", fallback, false},
		{"default", "darwin", fallback, false},
		{"cursor", "linux", []string{"cursor"}, false},
		{"code", "darwin", []string{"code"}, false},
		{"/path with spaces/editor", "linux", []string{"/path with spaces/editor"}, false},
		{"xcode", "darwin", []string{"/usr/bin/open", "-a", "Xcode"}, false},
		{"Xcode", "darwin", []string{"/usr/bin/open", "-a", "Xcode"}, false},
		{"Android Studio.app", "darwin", []string{"/usr/bin/open", "-a", "Android Studio.app"}, false},
		{"xcode", "linux", nil, true},
		{"Android Studio.app", "linux", nil, true},
	} {
		t.Run(tc.app+"/"+tc.goos, func(t *testing.T) {
			got, err := editorFor(project{App: tc.app}, fallback, tc.goos)
			if (err != nil) != tc.wantError || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestReplacePreservesAppAndAllowsReset(t *testing.T) {
	h := newHarness(t)
	h.want(t, 0, "add", "web", h.workspace, "--app=cursor")
	h.want(t, 1, "add", "web", h.folder, "--app", "code")
	if h.config(t).Projects["web"].App != "cursor" {
		t.Fatal("duplicate changed app")
	}
	h.want(t, 0, "add", "web", h.folder, "--replace")
	if h.config(t).Projects["web"].App != "cursor" {
		t.Fatal("path update lost app")
	}
	h.want(t, 0, "add", "web", h.folder, "--app", "code", "--replace")
	if h.config(t).Projects["web"].App != "code" {
		t.Fatal("app update failed")
	}
	h.want(t, 0, "add", "web", h.folder, "--app", "default", "--replace")
	if h.config(t).Projects["web"].App != "" {
		t.Fatal("app reset failed")
	}
}

func TestMixedConfigPreservesLegacyPathsAndProjectFields(t *testing.T) {
	h := newHarness(t)
	if err := os.MkdirAll(filepath.Dir(h.app.ConfigPath), 0700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(map[string]any{
		"projects": map[string]any{
			"legacy": h.folder,
			"web":    map[string]any{"path": h.workspace, "app": "cursor", "future": true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(h.app.ConfigPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	h.want(t, 0, "add", "web", h.folder, "--replace")
	after, err := os.ReadFile(h.app.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	var got struct{ Projects map[string]json.RawMessage }
	if err := json.Unmarshal(after, &got); err != nil {
		t.Fatal(err)
	}
	var legacy string
	if err := json.Unmarshal(got.Projects["legacy"], &legacy); err != nil || legacy != h.folder {
		t.Fatal("legacy path changed")
	}
	var updated struct {
		Path, App string
		Future    bool
	}
	if err := json.Unmarshal(got.Projects["web"], &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Path != h.folder || updated.App != "cursor" || !updated.Future {
		t.Fatalf("project data lost: %+v", updated)
	}
	h.want(t, 0, "list")
	if !strings.Contains(h.out.String(), "[cursor]") {
		t.Fatal("list does not show app")
	}
	h.app.Interactive = true
	h.app.In = strings.NewReader("\n")
	h.want(t, 0)
	if !strings.Contains(h.out.String(), "[cursor]") {
		t.Fatal("picker does not show app")
	}
}

func TestInvalidProjectObjectsAndAppArguments(t *testing.T) {
	for _, content := range []string{
		`{"projects":{"web":{"path":"/example","app":[]}}}`,
		`{"projects":{"web":{"path":"/example","app":null}}}`,
		`{"projects":{"web":{"path":"/example","app":""}}}`,
		`{"projects":{"web":{"app":"xcode"}}}`,
		`{"projects":{"web":{"path":null}}}`,
		`{"projects":{"web":null}}`,
	} {
		t.Run(content, func(t *testing.T) {
			h := newHarness(t)
			if err := os.MkdirAll(filepath.Dir(h.app.ConfigPath), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(h.app.ConfigPath, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			h.want(t, 1, "list")
		})
	}
	h := newHarness(t)
	for _, args := range [][]string{
		{"add", "web", h.folder, "--app"},
		{"add", "web", h.folder, "--app", "--replace"},
		{"add", "web", h.folder, "--app", ""},
		{"add", "web", h.folder, "--app="},
	} {
		h.want(t, 2, args...)
	}
}

func TestXcodeBundlesAccepted(t *testing.T) {
	h := newHarness(t)
	for _, name := range []string{"App.xcworkspace", "App.xcodeproj"} {
		path := filepath.Join(h.root, name)
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		h.want(t, 0, "add", strings.Split(name, ".")[1], path, "--app", "xcode")
	}
}
