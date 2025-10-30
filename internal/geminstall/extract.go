package geminstall

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// dirCache tracks created directories to avoid redundant MkdirAll syscalls
type dirCache struct {
	seen map[string]struct{}
	mu   sync.Mutex
}

func newDirCache(root string) *dirCache {
	dc := &dirCache{seen: make(map[string]struct{}, 256)}
	// Don't pre-mark root - let it be created properly via Ensure
	return dc
}

func (dc *dirCache) mark(path string) {
	if path == "" || path == "." {
		return
	}
	dc.seen[path] = struct{}{}
	// Mark all parent directories too
	parent := filepath.Dir(path)
	if parent != path && parent != "." {
		dc.mark(parent)
	}
}

func (dc *dirCache) Ensure(path string, mode os.FileMode) error {
	if path == "" || path == "." {
		return nil
	}

	dc.mu.Lock()
	_, exists := dc.seen[path]
	dc.mu.Unlock()

	if exists {
		return nil
	}

	if err := os.MkdirAll(path, mode); err != nil {
		return err
	}

	dc.mu.Lock()
	dc.mark(path)
	dc.mu.Unlock()

	return nil
}

// Buffer pool for file writes - reduces allocations and increases write size
var copyBufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 128<<10) // 128 KB buffer
		return &buf
	},
}

// ExtractMetadataOnly extracts only the metadata from a .gem file without extracting contents
// This is much faster than ExtractGemContents and useful for compatibility checks
func ExtractMetadataOnly(gemPath string) ([]byte, error) {
	file, err := os.Open(gemPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	tr := tar.NewReader(file)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch header.Name {
		case "metadata.gz":
			buf, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			return decompressMetadata(buf)
		case "metadata":
			return io.ReadAll(tr)
		}
	}

	return nil, fmt.Errorf("metadata not found in %s", gemPath)
}

// ExtractGemContents extracts a .gem file to the destination directory
// Returns the metadata YAML bytes
func ExtractGemContents(gemPath, destDir string) ([]byte, error) {
	file, err := os.Open(gemPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	tr := tar.NewReader(file)
	var dataFound bool
	var metadata []byte

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch header.Name {
		case "data.tar.gz":
			dataFound = true
			if err := extractDataTar(tr, destDir); err != nil {
				return nil, err
			}
		case "metadata.gz":
			buf, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			meta, err := decompressMetadata(buf)
			if err != nil {
				return nil, err
			}
			metadata = meta
		case "metadata":
			buf, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			metadata = buf
		case "data.tar.zst", "data.tar.bz2", "data.tar.xz":
			return nil, fmt.Errorf("unsupported gem payload compression (%s) for now", header.Name)
		}
	}

	if !dataFound {
		return nil, fmt.Errorf("data.tar.gz not found in %s", gemPath)
	}

	if metadata == nil {
		return nil, fmt.Errorf("metadata not found in %s", gemPath)
	}

	return metadata, nil
}

func extractDataTar(reader io.Reader, destDir string) error {
	gz, err := gzip.NewReader(reader)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() {
		_ = gz.Close()
	}()

	// Initialize directory cache to reduce MkdirAll syscalls
	cache := newDirCache(destDir)

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		targetPath := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			// Use tar header's mode when available
			mode := header.FileInfo().Mode() & os.ModePerm
			if mode == 0 {
				mode = 0o755
			}
			if err := cache.Ensure(targetPath, mode); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := cache.Ensure(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			if err := writeFileFromReader(targetPath, tr, header.FileInfo().Mode()); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := cache.Ensure(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			// Use os.Remove instead of os.RemoveAll - symlinks don't recurse
			if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
				return err
			}
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return err
			}
		default:
			// Ignore other entry types for now
		}
	}
}

// EnsureDir creates a directory if it doesn't exist
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func writeFileFromReader(path string, r io.Reader, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	// Use pooled buffer to reduce allocations and increase write size
	bufp := copyBufPool.Get().(*[]byte)
	defer copyBufPool.Put(bufp)

	if _, err := io.CopyBuffer(f, r, *bufp); err != nil {
		return err
	}
	return nil
}

func decompressMetadata(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decompress metadata: %w", err)
	}
	defer func() {
		_ = reader.Close()
	}()
	return io.ReadAll(reader)
}

// CopyGemToVendorCache copies a gem file to the vendor cache directory
func CopyGemToVendorCache(srcPath, destPath string) error {
	if err := EnsureDir(filepath.Dir(destPath)); err != nil {
		return err
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = src.Close()
	}()

	dest, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = dest.Close()
	}()

	if _, err := io.Copy(dest, src); err != nil {
		return err
	}

	return nil
}
