package fs

import (
	"io"
	"os"
	"path/filepath"
)

type Local struct{}

func (Local) Name() string { return "local" }

func (Local) Home() (string, error) { return os.UserHomeDir() }

func (Local) List(path string) ([]Entry, error) {
	dirents, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(dirents))
	for _, d := range dirents {
		out = append(out, Entry{
			Name:  d.Name(),
			Path:  filepath.Join(path, d.Name()),
			IsDir: d.IsDir(),
		})
	}
	Sort(out)
	return out, nil
}

func (Local) Download(srcPath, destDir string) error {
	return copyTree(srcPath, filepath.Join(destDir, filepath.Base(srcPath)))
}

func copyTree(src, dest string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return copyFile(src, dest)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := copyTree(filepath.Join(src, e.Name()), filepath.Join(dest, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}
