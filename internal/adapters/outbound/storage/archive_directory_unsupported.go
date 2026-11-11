//go:build !linux

package storage

import (
	"errors"
	"os"
)

// Fail closed where descriptor-relative no-follow traversal is not implemented.
func openArchiveDirectory(string) (*os.Root, *os.File, error) {
	return nil, nil, errors.New("secure local source archive requires Linux")
}
func openExistingOriginal(*os.File, string) (*os.File, error) {
	return nil, errors.New("secure local source archive requires Linux")
}
