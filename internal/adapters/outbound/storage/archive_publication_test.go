//go:build linux

package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestArchivePublicationWaitsForCompleteContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive")
	root, dir, err := openArchiveDirectory(path)
	require.NoError(t, err)
	defer func() { _ = root.Close() }()
	defer func() { _ = dir.Close() }()
	content := []byte("synthetic complete bytes")
	digest := sha256.Sum256(content)
	name := hex.EncodeToString(digest[:]) + ".source"
	reader, writer := io.Pipe()
	finished := make(chan error, 1)
	go func() { finished <- storeOriginal(root, name, reader); _ = reader.Close() }()
	_, err = writer.Write(content[:4])
	require.NoError(t, err)
	_, statErr := root.Lstat(name)
	// Finish the blocked writer before any fatal assertion.
	_, writeErr := writer.Write(content[4:])
	closeErr := writer.Close()
	storeErr := <-finished
	require.True(t, os.IsNotExist(statErr), "partial original must not occupy the final digest")
	require.NoError(t, writeErr)
	require.NoError(t, closeErr)
	require.NoError(t, storeErr)
	got, err := root.ReadFile(name)
	require.NoError(t, err)
	require.Equal(t, content, got)
}

func TestArchiveFailedPublicationLeavesDigestAvailable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive")
	root, dir, err := openArchiveDirectory(path)
	require.NoError(t, err)
	defer func() { _ = root.Close() }()
	defer func() { _ = dir.Close() }()
	content := []byte("synthetic retry")
	digest := sha256.Sum256(content)
	hash := hex.EncodeToString(digest[:])
	// A different operation's incomplete temporary file must not be cleaned up.
	require.NoError(t, root.WriteFile(".source-other.tmp", []byte("other"), 0600))
	source := io.MultiReader(bytes.NewReader(content[:3]), brokenReader{})
	require.Error(t, storeOriginal(root, hash+".source", source))
	_, err = root.Lstat(hash + ".source")
	require.True(t, os.IsNotExist(err), "failed copy must never poison final digest")
	_, err = NewLocalSourceArchive(path).Retain(context.Background(), content)
	require.NoError(t, err)
	entries, err := os.ReadDir(path)
	require.NoError(t, err)
	require.Len(t, entries, 2, "only completed original and somebody else's temp remain")
}

type brokenReader struct{}

func TestArchiveRemainsAnchoredAfterAncestorSymlinkSwap(t *testing.T) {
	for _, swapFinal := range []bool{false, true} {
		t.Run(map[bool]string{false: "ancestor", true: "archive"}[swapFinal], func(t *testing.T) {
			base := t.TempDir()
			parent := filepath.Join(base, "outside")
			path := filepath.Join(parent, "archive")
			root, dir, err := openArchiveDirectory(path)
			require.NoError(t, err)
			defer func() { _ = root.Close() }()
			defer func() { _ = dir.Close() }()
			checkout := filepath.Join(base, "checkout")
			require.NoError(t, os.Mkdir(checkout, 0700))
			require.NoError(t, os.Mkdir(filepath.Join(checkout, ".git"), 0700))
			require.NoError(t, os.Mkdir(filepath.Join(checkout, "archive"), 0700))
			swap := parent
			if swapFinal {
				swap = path
			}
			moved := swap + "-original"
			require.NoError(t, os.Rename(swap, moved))
			require.NoError(t, os.Symlink(checkout, swap))
			content := []byte("synthetic pinned content")
			require.NoError(t, storeOriginal(root, "digest.source", bytes.NewReader(content)))
			require.NoError(t, dir.Sync())
			expected := filepath.Join(moved, "archive", "digest.source")
			if swapFinal {
				expected = filepath.Join(moved, "digest.source")
			}
			got, err := os.ReadFile(expected) // #nosec G304 -- path constructed exclusively under t.TempDir.
			require.NoError(t, err)
			require.Equal(t, content, got)
			entries, err := os.ReadDir(checkout)
			require.NoError(t, err)
			require.Len(t, entries, 2, "replacement checkout received no files")
			entries, err = os.ReadDir(filepath.Join(checkout, "archive"))
			require.NoError(t, err)
			require.Empty(t, entries)
			_, err = NewLocalSourceArchive(path).Retain(context.Background(), content)
			require.Error(t, err, "a new operation must reject the replacement symlink")
		})
	}
}

func TestArchiveConcurrentRetainWhileAnotherCopyIsIncomplete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive")
	root, dir, err := openArchiveDirectory(path)
	require.NoError(t, err)
	defer func() { _ = root.Close() }()
	defer func() { _ = dir.Close() }()
	content := []byte("synthetic overlapping writes")
	digest := sha256.Sum256(content)
	hash := hex.EncodeToString(digest[:])
	reader, writer := io.Pipe()
	finished := make(chan error, 1)
	go func() { finished <- storeOriginal(root, hash+".source", reader); _ = reader.Close() }()
	_, err = writer.Write(content[:4])
	require.NoError(t, err)
	concurrentHash, concurrentErr := NewLocalSourceArchive(path).Retain(context.Background(), content)
	_, writeErr := writer.Write(content[4:])
	closeErr := writer.Close()
	firstErr := <-finished
	require.NoError(t, concurrentErr, "a partial competing copy must not make valid retention fail")
	require.Equal(t, hash, concurrentHash)
	require.NoError(t, writeErr)
	require.NoError(t, closeErr)
	require.ErrorIs(t, firstErr, os.ErrExist, "completed competitor wins without replacement")
	require.NoError(t, verifyOriginal(dir, hash+".source", content))
	entries, err := os.ReadDir(path)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func (brokenReader) Read([]byte) (int, error) { return 0, errors.New("synthetic write interruption") }
