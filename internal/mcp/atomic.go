package mcp

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jyablonski/arc/internal/filemode"
)

// writeFileAtomic replaces a file in one step: write a temp file in the same
// directory, fsync it, then rename over the target.
//
// This matters more here than anywhere else in arc. Unlike the symlink stores
// behind `skills` and `rules`, syncing MCP config rewrites files the AI tools
// own and keep large amounts of unrelated state in — ~/.claude.json is ~80KB of
// session history. A plain truncate-and-write leaves a window where an
// interrupt destroys all of it; rename(2) has no such window.
//
// The mode of an existing file is preserved, so arc never loosens permissions
// on a file a tool created restrictively.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, filemode.Dir); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}

	tmp, err := os.CreateTemp(dir, ".arc-mcp-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	// Any early return below leaves the target untouched; drop the temp file.
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod %s: %w", tmpName, err)
	}
	// Flush to disk before the rename so a crash cannot leave the target
	// pointing at a file whose contents never landed.
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmpName, path, err)
	}
	return nil
}
