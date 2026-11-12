package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func worktreeFixture(t *testing.T) (string, string) {
	t.Helper()
	root, _ := filepath.EvalSymlinks(t.TempDir())
	repo := filepath.Join(root, "main repo")
	if err := os.Mkdir(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	gitTest(t, repo, "init", "-b", "main")
	gitTest(t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "initial")
	other := filepath.Join(root, "feature 'quote $(touch PWNED)")
	gitTest(t, repo, "worktree", "add", "-b", "feature/review", other)
	return repo, other
}

func TestDiscoverRealWorktreesAndDeduplicate(t *testing.T) {
	repo, other := worktreeFixture(t)
	trees, err := discoverWorktrees([]string{repo, other, t.TempDir()})
	if err != nil || len(trees) != 2 {
		t.Fatalf("discovery: %+v, %v", trees, err)
	}
	found := false
	for _, tree := range trees {
		if tree.Path == other {
			found = tree.Branch == "feature/review" && tree.Repo == filepath.Base(repo)
		}
	}
	if !found {
		t.Fatal("feature checkout not discovered")
	}
	gitTest(t, repo, "worktree", "lock", "--reason", "keep", other)
	trees, err = discoverWorktrees([]string{repo})
	if err != nil {
		t.Fatal(err)
	}
	for _, tree := range trees {
		if tree.Path == other && (!tree.Locked || !strings.Contains(tree.label(), "[locked]")) {
			t.Fatal("lock metadata lost")
		}
	}
}

func TestWorkspaceFolderDiscovery(t *testing.T) {
	repo, other := worktreeFixture(t)
	workspace := filepath.Join(filepath.Dir(repo), "example.code-workspace")
	data, _ := json.Marshal(map[string]any{"folders": []map[string]string{{"path": filepath.Base(repo)}, {"path": other}}})
	if err := os.WriteFile(workspace, data, 0o600); err != nil {
		t.Fatal(err)
	}
	folders, err := projectFolders(workspace)
	if err != nil || len(folders) != 2 || folders[0] != repo {
		t.Fatalf("folders: %v, %v", folders, err)
	}
	trees, err := discoverWorktrees(folders)
	if err != nil || len(trees) != 2 {
		t.Fatal(trees, err)
	}
}

func TestPorcelainDetachedAndUnsupportedPaths(t *testing.T) {
	data := []byte("worktree /tmp/example with spaces\x00HEAD abc\x00detached\x00locked reason\x00prunable reason\x00\x00")
	trees, err := parseWorktrees(data)
	if err != nil || len(trees) != 1 || !trees[0].Detached || !trees[0].Prunable || !trees[0].Locked {
		t.Fatal(trees, err)
	}
	if _, err := parseWorktrees([]byte("worktree /tmp/line\nbreak\x00\x00")); err == nil {
		t.Fatal("accepted ambiguous newline path")
	}
}
