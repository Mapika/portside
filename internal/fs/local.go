package fs

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
		e := Entry{
			Name:  d.Name(),
			Path:  filepath.Join(path, d.Name()),
			IsDir: d.IsDir(),
		}
		if info, infoErr := d.Info(); infoErr == nil {
			e.Size = info.Size()
			e.ModTime = info.ModTime()
		}
		out = append(out, e)
	}
	Sort(out)
	return out, nil
}

// Exec runs name with args (argv form, no shell) and returns combined output.
func (Local) Exec(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

func (Local) Download(srcPath, destDir string) error {
	return copyTree(srcPath, filepath.Join(destDir, filepath.Base(srcPath)))
}

func (Local) Upload(localSrc, destDir string) error {
	return copyTree(localSrc, filepath.Join(destDir, filepath.Base(localSrc)))
}

func (Local) Rename(oldPath, newName string) error {
	if strings.ContainsAny(newName, "/\\") || newName == "." || newName == ".." || newName == "" {
		return fmt.Errorf("invalid name: %q", newName)
	}
	return os.Rename(oldPath, filepath.Join(filepath.Dir(oldPath), newName))
}

func (Local) Remove(path string) error {
	return os.RemoveAll(path)
}

func (Local) Mkdir(path string) error {
	return os.Mkdir(path, 0o755)
}

func copyTree(src, dest string) error {
	info, err := os.Lstat(src)
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
