package gitlog

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommitStagedOnlyCommitsStaged(t *testing.T) {
	root := repoWithCommits(t) // has repo-local identity + initial commits
	ctx := context.Background()
	os.WriteFile(filepath.Join(root, "staged.txt"), []byte("s"), 0o644)
	os.WriteFile(filepath.Join(root, "unstaged.txt"), []byte("u"), 0o644)
	if err := StageFile(ctx, root, "staged.txt"); err != nil {
		t.Fatal(err)
	}
	if err := CommitStaged(ctx, root, "only staged"); err != nil {
		t.Fatalf("CommitStaged: %v", err)
	}
	// The commit must contain staged.txt but NOT unstaged.txt.
	out, _ := exec.Command("git", "-C", root, "show", "--name-only", "--format=", "HEAD").Output()
	files := string(out)
	if !strings.Contains(files, "staged.txt") {
		t.Errorf("commit should include staged.txt, got: %q", files)
	}
	if strings.Contains(files, "unstaged.txt") {
		t.Errorf("commit must NOT include unstaged.txt, got: %q", files)
	}
	// unstaged.txt should still be an untracked/uncommitted change.
	st, _ := exec.Command("git", "-C", root, "status", "--porcelain").Output()
	if !strings.Contains(string(st), "unstaged.txt") {
		t.Errorf("unstaged.txt should remain uncommitted")
	}
}
