package storage_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/storage"
	"github.com/stretchr/testify/require"
)

func TestLocalArchiveRejectsGitThroughAncestorSymlink(t *testing.T) {
	for _, gitFile := range []bool{false, true} {
		t.Run(map[bool]string{false: "directory", true: "worktree-file"}[gitFile], func(t *testing.T) {
			base := t.TempDir()
			checkout := filepath.Join(base, "checkout")
			require.NoError(t, os.Mkdir(checkout, 0700))
			if gitFile {
				require.NoError(t, os.WriteFile(filepath.Join(checkout, ".git"), []byte("gitdir: synthetic"), 0600))
			} else {
				require.NoError(t, os.Mkdir(filepath.Join(checkout, ".git"), 0700))
			}
			link := filepath.Join(base, "link")
			require.NoError(t, os.Symlink(checkout, link))
			_, err := storage.NewLocalSourceArchive(filepath.Join(link, "archive")).Retain(context.Background(), []byte("synthetic"))
			require.Error(t, err, "must reject checkout through symlink ancestor")
			_, err = os.Stat(filepath.Join(checkout, "archive"))
			require.True(t, os.IsNotExist(err), "must refuse before creating anything in checkout")
		})
	}
}

func TestLocalArchiveBoundaryCases(t *testing.T) {
	for _, kind := range []string{"relative", "permissions", "final-directory-symlink", "ancestor-symlink", "git-directory", "git-file", "git-dangling-symlink", "final-file-symlink", "file-permissions", "final-file-directory"} {
		t.Run(kind, func(t *testing.T) {
			base := t.TempDir()
			path := filepath.Join(base, "archive")
			content := []byte("synthetic boundary")
			digest := sha256.Sum256(content)
			name := hex.EncodeToString(digest[:]) + ".source"
			switch kind {
			case "relative":
				path = "relative-archive"
			case "permissions":
				require.NoError(t, os.Mkdir(path, 0755)) // #nosec G301 -- deliberately insecure synthetic fixture must be rejected.
			case "final-directory-symlink", "ancestor-symlink":
				real := filepath.Join(base, "real")
				require.NoError(t, os.Mkdir(real, 0700))
				require.NoError(t, os.Symlink(real, path))
				if kind == "ancestor-symlink" {
					path = filepath.Join(path, "nested")
				}
			case "git-directory", "git-file", "git-dangling-symlink":
				git := filepath.Join(base, ".git")
				switch kind {
				case "git-directory":
					require.NoError(t, os.Mkdir(git, 0700))
				case "git-file":
					require.NoError(t, os.WriteFile(git, []byte("gitdir: synthetic"), 0600))
				default:
					require.NoError(t, os.Symlink("missing", git))
				}
			case "final-file-symlink", "file-permissions", "final-file-directory":
				require.NoError(t, os.Mkdir(path, 0700))
				target := filepath.Join(path, name)
				switch kind {
				case "final-file-symlink":
					real := filepath.Join(path, "real-source")
					require.NoError(t, os.WriteFile(real, content, 0600))
					require.NoError(t, os.Symlink("real-source", target))
				case "file-permissions":
					require.NoError(t, os.WriteFile(target, content, 0644)) // #nosec G306 -- deliberately insecure synthetic fixture must be rejected.
				default:
					require.NoError(t, os.Mkdir(target, 0700))
				}
			}
			_, err := storage.NewLocalSourceArchive(path).Retain(context.Background(), content)
			require.Error(t, err)
		})
	}
}

func TestLocalArchiveConcurrentIdenticalRetains(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "archive")
	content := make([]byte, 1024*1024)
	copy(content, []byte("synthetic concurrent original"))
	const workers = 24
	start := make(chan struct{})
	results := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := storage.NewLocalSourceArchive(dir).Retain(context.Background(), content)
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err, "identical concurrent retention must succeed")
	}
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "no temporary files left behind")
	got, err := os.ReadFile(filepath.Join(dir, entries[0].Name())) // #nosec G304 -- enumerated file in test-owned t.TempDir.
	require.NoError(t, err)
	require.Equal(t, content, got)
}
