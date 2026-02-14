package util

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

var ErrPathOutsideRoot = errors.New("path outside repo root")

// ResolvePathWithinRoot resolves a relative path under root and rejects traversal.
// If the resolved path exists, we also resolve symlinks and ensure it still stays under root.
func ResolvePathWithinRoot(root string, rel string) (string, error) {
	if rel == "" {
		return "", errors.New("empty path")
	}
	if filepath.IsAbs(rel) {
		return "", ErrPathOutsideRoot
	}

	cleanRel := filepath.Clean(rel)
	if cleanRel == "." || cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(os.PathSeparator)) {
		return "", ErrPathOutsideRoot
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absTarget, err := filepath.Abs(filepath.Join(absRoot, cleanRel))
	if err != nil {
		return "", err
	}
	if !isWithin(absRoot, absTarget) {
		return "", ErrPathOutsideRoot
	}

	// Best-effort symlink escape protection for existing paths.
	if _, err := os.Stat(absTarget); err == nil {
		rootReal, rerr := filepath.EvalSymlinks(absRoot)
		if rerr != nil {
			rootReal = absRoot
		}
		targetReal, terr := filepath.EvalSymlinks(absTarget)
		if terr != nil {
			targetReal = absTarget
		}
		if !isWithin(rootReal, targetReal) {
			return "", ErrPathOutsideRoot
		}
	}

	return absTarget, nil
}

func isWithin(root string, target string) bool {
	root = filepath.Clean(root)
	target = filepath.Clean(target)

	if root == target {
		return true
	}
	prefix := root + string(os.PathSeparator)
	return strings.HasPrefix(target, prefix)
}
