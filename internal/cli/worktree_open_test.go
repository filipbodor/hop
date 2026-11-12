package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorktreeListSelectionAndCancellation(t *testing.T) {
	h := newHarness(t)
	repo, other := worktreeFixture(t)
	t.Chdir(t.TempDir())
	h.want(t, 0, "add", "example", repo)
	h.want(t, 0, "worktree", "--list")
	if !strings.Contains(h.out.String(), "feature/review") || !strings.Contains(h.out.String(), other) {
		t.Fatal(h.out.String())
	}
	h.want(t, 0, "worktree", "--path-only", "feature/review")
	if strings.TrimSpace(h.out.String()) != other {
		t.Fatal(h.out.String())
	}
	h.want(t, 1, "worktree", "--path-only")
	if !strings.Contains(h.stderr.String(), "multiple matching") {
		t.Fatal(h.stderr.String())
	}
	h.app.Interactive = true
	h.app.In = strings.NewReader("\n")
	h.want(t, 0, "worktree")
	if h.out.Len() != 0 || !strings.Contains(h.stderr.String(), "feature/review") {
		t.Fatal("picker UI must use stderr and cancellation must not emit a path")
	}
	h.app.In = strings.NewReader("1\n")
	h.want(t, 0, "worktree", "--path-only")
	if !filepath.IsAbs(strings.TrimSpace(h.out.String())) {
		t.Fatal(h.out.String())
	}
	if err := os.RemoveAll(other); err != nil {
		t.Fatal(err)
	}
	h.want(t, 0, "worktree", "--list", "feature/review")
	if !strings.Contains(h.out.String(), "[missing]") {
		t.Fatal(h.out.String())
	}
	h.want(t, 1, "worktree", "--path-only", "feature/review")
}

func TestWorktreeLaunchUsesLiteralPathAndKeepsStdoutClean(t *testing.T) {
	h := newHarness(t)
	repo, other := worktreeFixture(t)
	t.Chdir(t.TempDir())
	tools := t.TempDir()
	capture := filepath.Join(t.TempDir(), "calls")
	// Both GUI commands are harmless stubs; no actual applications are opened.
	for _, name := range []string{"code", "smerge"} {
		script := "#!/bin/sh\nprintf '%s\\n' \"$PWD\" \"$@\" >> \"$HOP_WORKTREE_CAPTURE\"\necho GUI-output\n"
		if err := os.WriteFile(filepath.Join(tools, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HOP_WORKTREE_CAPTURE", capture)
	h.want(t, 0, "editor", "code")
	h.want(t, 0, "worktree", "--repo", repo, "feature/review")
	if strings.TrimSpace(h.out.String()) != other {
		t.Fatal("GUI output polluted destination", h.out.String())
	}
	data, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), other) != 4 || !strings.Contains(string(data), "--new-window") || !strings.Contains(h.stderr.String(), "GUI-output") {
		t.Fatal("apps did not receive the same literal cwd and path", string(data), h.stderr.String())
	}
	if _, err := os.Stat(filepath.Join(other, "PWNED")); !os.IsNotExist(err) {
		t.Fatal("path was evaluated")
	}
}

func TestExistingRegistrationsKeepNewCommandNames(t *testing.T) {
	h := newHarness(t)
	data, _ := json.Marshal(map[string]any{"projects": map[string]string{"worktree": h.folder, "init": h.folder}})
	if err := os.MkdirAll(filepath.Dir(h.app.ConfigPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(h.app.ConfigPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	h.want(t, 0, "list")
	h.want(t, 1, "add", "worktree", h.folder)
	h.want(t, 0, "init", "zsh")
	if !strings.Contains(h.out.String(), "function hwt()") {
		t.Fatal(h.out.String())
	}
}

func TestZshWorktreeHelperChangesTerminalAndListsWithoutCd(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("requires Zsh")
	}
	binary := filepath.Join(t.TempDir(), "hop")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/hop")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	repo, other := worktreeFixture(t)
	configHome := t.TempDir()
	script := `eval "$(hop init zsh)"
initial="$PWD"
hwt --repo "$REPO" --list >/dev/null || exit 41
[[ "$PWD" == "$initial" ]] || exit 42
hwt --repo "$REPO" --path-only feature/review || exit 43
[[ "$PWD" == "$EXPECTED" ]] || exit 44
`
	cmd := exec.Command(zsh, "-d", "-f", "-c", script)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+configHome, "REPO="+repo, "EXPECTED="+other,
		"PATH="+filepath.Dir(binary)+string(os.PathListSeparator)+os.Getenv("PATH"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("shell helper: %v\n%s", err, out)
	}
}
