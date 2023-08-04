package atomic

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	// DefaultFilePerm are the default file permission to use when the permissions
	// cannot be determined from the existing file.
	DefaultFilePerm os.FileMode = 0o644

	// DefaultFolderPerm is the default permission to use when creating parent
	// directories, if they do not already exist.
	DefaultFolderPerm os.FileMode = 0o755
)

// AtomicWrite accepts a destination path and an reader. It copies the contents
// to a TempFile on disk, returning if any errors occur.
//
// If the parent destination directory does not exist, it will be created
// automatically with permissions 0755. To use a different permission, create
// the directory first or use `chmod`.
//
// If the destination path exists, all attempts will be made to preserve the
// existing file permissions. If those permissions cannot be read, an error is
// returned. If the file does not exist, it will be created automatically with
// permissions 0644. To use a different permission, create the destination file
// first.
//
// If no errors occur, the Tempfile is "renamed" (moved) to the destination
// path. On Unix systems, this is guaranteed to be atomic. On Windows, it will
// be atomic if the files are on the same volume, but cannot be guaranteed if
// the file crosses volumes. For more information, see:
//
//	https://github.com/golang/go/issues/22397#issuecomment-498856679
func Write(src, dst string, r io.Reader) error {
	parent := filepath.Dir(dst)
	if _, err := os.Stat(parent); os.IsNotExist(err) {
		if err := os.MkdirAll(parent, DefaultFolderPerm); err != nil {
			return fmt.Errorf("failed to make parent directory: %w", err)
		}
	}

	f, err := os.CreateTemp(parent, "ratchet-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())

	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}

	// Get the current permissions on the dst file. If the dst file does not
	// exist, get the permissions on the original src file.
	currentInfo, err := os.Stat(dst)
	if os.IsNotExist(err) {
		currentInfo, err = os.Stat(src)
	}
	if err != nil {
		return fmt.Errorf("failed to get permissions on file: %w", err)
	}

	if err := preserveOwnership(f.Name(), currentInfo); err != nil {
		return fmt.Errorf("failed to preserve file permissions: %w", err)
	}

	if err := os.Rename(f.Name(), dst); err != nil {
		return fmt.Errorf("failed to atomically write: %w", err)
	}

	return nil
}
