package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
)

// LocalSourceArchive retains exact source bytes in a private local directory.
type LocalSourceArchive struct{ directory string }

// NewLocalSourceArchive configures the operator-selected private archive path.
func NewLocalSourceArchive(directory string) *LocalSourceArchive {
	return &LocalSourceArchive{directory: directory}
}

// Validate verifies the same anchored boundary used by Retain, without retaining
// source bytes. Retain always revalidates; this is not a reusable authorization.
func (a *LocalSourceArchive) Validate() error {
	root, directory, err := openArchiveDirectory(a.directory)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close(); _ = directory.Close() }()
	return nil
}

// Retain stores the original bytes once in a private content-addressed archive.
// Existing mismatches are errors, never overwritten or removed automatically.
func (a *LocalSourceArchive) Retain(ctx context.Context, content []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	root, directory, err := openArchiveDirectory(a.directory)
	if err != nil {
		return "", err
	}
	defer func() { _ = root.Close(); _ = directory.Close() }()
	digest := sha256.Sum256(content)
	hash := hex.EncodeToString(digest[:])
	target := hash + ".source"
	err = storeOriginal(root, target, bytes.NewReader(content))
	if errors.Is(err, os.ErrExist) {
		err = verifyOriginal(directory, target, content)
	}
	if err != nil {
		return "", err
	}
	if err = directory.Sync(); err != nil {
		return "", errors.New("source directory sync failed; no import permitted")
	}
	return hash, nil
}

// storeOriginal publishes a fully written and fsynced inode using an atomic,
// no-replace hard link. A competing writer sees either no digest or complete
// bytes; EEXIST is verified by Retain, never treated as an incomplete write.
func storeOriginal(root *os.Root, target string, source io.Reader) error {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	temporary := ".source-" + hex.EncodeToString(random[:]) + ".tmp"
	file, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	// This operation owns only this exclusive, random temporary name. Never
	// remove target (even after a failed sync), or enumerate/clean other temps.
	defer func() { _ = file.Close(); _ = root.Remove(temporary) }()
	if _, err = io.Copy(file, source); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return root.Link(temporary, target)
}

func verifyOriginal(directory *os.File, name string, content []byte) error {
	file, err := openExistingOriginal(directory, name)
	if err != nil {
		return errors.New("retained source is not a private regular file")
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return errors.New("retained source is not a private regular file")
	}
	existing, err := io.ReadAll(io.LimitReader(file, int64(len(content))+1))
	if err != nil || !bytes.Equal(existing, content) {
		return errors.New("retained original does not match; manual recovery required")
	}
	return nil
}
