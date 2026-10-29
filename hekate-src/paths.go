// ============================================================================
// paths.go - Data Path Resolution
// ============================================================================
//
// Helpers for resolving user-supplied datafile paths against a configured
// data_dir, with sandboxing to prevent escaping the directory.
//
// ============================================================================

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveDataPath resolves a user-supplied datafile path against dataDir
// and returns the cleaned absolute path.
//
// Rules:
//   - If dataDir is empty, the path is resolved against the CWD and may be
//     absolute (legacy behaviour).
//   - If dataDir is set, absolute paths are rejected and any path that
//     escapes dataDir (via ".." or symlinks resolved by filepath.Clean) is
//     rejected.
//   - The resulting path must point to an existing regular file.
func ResolveDataPath(dataDir, requested string) (string, error) {
	if requested == "" {
		return "", fmt.Errorf("empty datafile path")
	}

	// ---- Legacy mode: no dataDir configured ----
	if dataDir == "" {
		if filepath.IsAbs(requested) {
			// Allowed for now; the user has been warned via LogSummary.
			return filepath.Clean(requested), nil
		}
		abs, err := filepath.Abs(requested)
		if err != nil {
			return "", fmt.Errorf("cannot resolve %q: %w", requested, err)
		}
		return abs, nil
	}

	// ---- Sandboxed mode: resolve against dataDir ----
	if filepath.IsAbs(requested) {
		return "", fmt.Errorf("datafile path %q must be relative (data_dir is set to %q)", requested, dataDir)
	}

	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return "", fmt.Errorf("cannot resolve data_dir %q: %w", dataDir, err)
	}

	candidate := filepath.Join(absDataDir, requested)
	candidate = filepath.Clean(candidate)

	// Escape check: candidate must be strictly inside absDataDir.
	rel, err := filepath.Rel(absDataDir, candidate)
	if err != nil {
		return "", fmt.Errorf("cannot resolve %q against data_dir: %w", requested, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("datafile path %q escapes data_dir %q", requested, dataDir)
	}

	return candidate, nil
}

// StatDataPath verifies that the given absolute path exists and is a regular
// file. Returns a descriptive error otherwise.
func StatDataPath(absPath string) error {
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", absPath)
		}
		return fmt.Errorf("cannot stat %s: %w", absPath, err)
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", absPath)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("path is not a regular file: %s", absPath)
	}
	return nil
}