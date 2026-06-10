package fs

import (
	"io"
	"os"
	"path" // remote paths are always POSIX
	"path/filepath"

	"github.com/pkg/sftp"
)

type SFTP struct {
	client *sftp.Client
	host   string
}

func NewSFTP(client *sftp.Client, host string) *SFTP {
	return &SFTP{client: client, host: host}
}

func (s *SFTP) Name() string { return s.host }

func (s *SFTP) Home() (string, error) { return s.client.Getwd() }

func (s *SFTP) List(p string) ([]Entry, error) {
	infos, err := s.client.ReadDir(p)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(infos))
	for _, fi := range infos {
		out = append(out, Entry{
			Name:  fi.Name(),
			Path:  path.Join(p, fi.Name()),
			IsDir: fi.IsDir(),
		})
	}
	Sort(out)
	return out, nil
}

func (s *SFTP) Download(srcPath, destDir string) error {
	fi, err := s.client.Stat(srcPath)
	if err != nil {
		return err
	}
	dest := filepath.Join(destDir, path.Base(srcPath))
	if !fi.IsDir() {
		return s.downloadFile(srcPath, dest)
	}
	walker := s.client.Walk(srcPath)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(srcPath, walker.Path())
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if walker.Stat().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		} else if err := s.downloadFile(walker.Path(), target); err != nil {
			return err
		}
	}
	return nil
}

func (s *SFTP) downloadFile(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	in, err := s.client.Open(src)
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
