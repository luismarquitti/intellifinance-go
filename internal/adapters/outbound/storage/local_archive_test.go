package storage_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/storage"
	"github.com/stretchr/testify/require"
)

func TestLocalSourceArchiveRetainsImmutablePrivateOriginal(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "archive")
	archive := storage.NewLocalSourceArchive(dir)
	original := []byte("synthetic original bytes\n")
	hash, err := archive.Retain(context.Background(), original)
	require.NoError(t, err)
	expected := sha256.Sum256(original)
	require.Equal(t, hex.EncodeToString(expected[:]), hash)
	target := filepath.Join(dir, hash+".source")
	//nolint:gosec // G304: synthetic file in t.TempDir with a computed digest basename.
	content, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, original, content)
	info, err := os.Stat(target)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	info, err = os.Stat(dir)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), info.Mode().Perm())
	again, err := archive.Retain(context.Background(), original)
	require.NoError(t, err)
	require.Equal(t, hash, again)
	require.NoError(t, os.WriteFile(target, []byte("synthetic tampering"), 0o600))
	_, err = archive.Retain(context.Background(), original)
	require.Error(t, err, "archive must never overwrite an inconsistent retained original")
	//nolint:gosec // G304: the same synthetic t.TempDir target, never an external source.
	content, err = os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, []byte("synthetic tampering"), content)
}
