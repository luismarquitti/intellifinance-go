//go:build linux

package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"syscall"
)

// openArchiveDirectory walks from / using no-follow, descriptor-relative opens.
// No pathname check is reused for a subsequent unanchored filesystem operation.
// All symlinks are deliberately refused, even ones pointing outside Git.
func openArchiveDirectory(path string) (*os.Root, *os.File, error) {
	if !filepath.IsAbs(path) {
		return nil, nil, errors.New("source archive requires an absolute private path")
	}
	fd, openErr := syscall.Open("/", syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if openErr != nil {
		return nil, nil, openErr
	}
	dir := os.NewFile(uintptr(fd), "/")
	defer func() {
		if dir != nil {
			_ = dir.Close()
		}
	}()
	checkGit := func() error {
		gitFD, err := syscall.Openat(int(dir.Fd()), ".git", syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
		if err == nil {
			_ = syscall.Close(gitFD)
			return errors.New("original source archive cannot be inside any Git checkout")
		}
		if !errors.Is(err, syscall.ENOENT) {
			return errors.New("cannot verify source archive Git boundary")
		}
		return nil
	}
	if err := checkGit(); err != nil {
		return nil, nil, err
	}
	for _, component := range strings.Split(strings.TrimPrefix(filepath.Clean(path), "/"), "/") {
		if component == "" {
			continue
		}
		next, err := syscall.Openat(int(dir.Fd()), component, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		if errors.Is(err, syscall.ENOENT) {
			if err = syscall.Mkdirat(int(dir.Fd()), component, 0700); err != nil && !errors.Is(err, syscall.EEXIST) {
				return nil, nil, errors.New("cannot create private source archive")
			}
			next, err = syscall.Openat(int(dir.Fd()), component, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		}
		if err != nil {
			return nil, nil, errors.New("source archive path must contain only non-symlink directories")
		}
		_ = dir.Close()
		dir = os.NewFile(uintptr(next), component)
		if err := checkGit(); err != nil {
			return nil, nil, err
		}
	}
	info, err := dir.Stat()
	if err != nil || info.Mode().Perm() != 0700 {
		return nil, nil, errors.New("source archive must be a private non-symlink directory")
	}
	// /proc/self/fd names the already verified descriptor, not the supplied path.
	// Root then pins this directory independently across ancestor symlink swaps.
	root, err := os.OpenRoot(fmt.Sprintf("/proc/self/fd/%d", dir.Fd()))
	if err != nil {
		return nil, nil, errors.New("cannot anchor private source archive")
	}
	result := dir
	dir = nil
	return root, result, nil
}

func openExistingOriginal(directory *os.File, name string) (*os.File, error) {
	// Root.OpenFile follows in-root symlinks, even with O_NOFOLLOW on Go 1.25.
	// Use a single-component openat so the kernel enforces no-follow itself.
	fd, err := syscall.Openat(int(directory.Fd()), name, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), name), nil
}
