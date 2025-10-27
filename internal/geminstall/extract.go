package geminstall

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ExtractGemContents extracts a .gem file to the destination directory
// Returns the metadata YAML bytes
func ExtractGemContents(gemPath, destDir string) ([]byte, error) {
	file, err := os.Open(gemPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

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
	defer gz.Close()

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
			if err := EnsureDir(targetPath); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := EnsureDir(filepath.Dir(targetPath)); err != nil {
				return err
			}
			if err := writeFileFromReader(targetPath, tr, header.FileInfo().Mode()); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := EnsureDir(filepath.Dir(targetPath)); err != nil {
				return err
			}
			if err := os.RemoveAll(targetPath); err != nil {
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
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return err
	}
	return nil
}

func decompressMetadata(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decompress metadata: %w", err)
	}
	defer reader.Close()
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
	defer src.Close()

	dest, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dest.Close()

	if _, err := io.Copy(dest, src); err != nil {
		return err
	}

	return nil
}
