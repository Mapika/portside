package fs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path" // remote paths are always POSIX
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SFTP struct {
	client *sftp.Client
	sshc   *ssh.Client
	host   string
}

func NewSFTP(client *sftp.Client, host string) *SFTP {
	return &SFTP{client: client, host: host}
}

// NewSFTPWithExec creates an SFTP backend that also supports Exec via the
// supplied ssh.Client.
func NewSFTPWithExec(client *sftp.Client, sshc *ssh.Client, host string) *SFTP {
	return &SFTP{client: client, sshc: sshc, host: host}
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
			Name:    fi.Name(),
			Path:    path.Join(p, fi.Name()),
			IsDir:   fi.IsDir(),
			Size:    fi.Size(),
			ModTime: fi.ModTime(),
		})
	}
	Sort(out)
	return out, nil
}

// Exec runs name with args on the remote machine via a new SSH session. It
// returns an error if no ssh.Client was provided (see NewSFTPWithExec).
func (s *SFTP) Exec(name string, args ...string) ([]byte, error) {
	if s.sshc == nil {
		return nil, errors.New("sftp: Exec called without an ssh.Client")
	}
	sess, err := s.sshc.NewSession()
	if err != nil {
		return nil, err
	}
	defer sess.Close()
	return sess.Output(shellJoin(name, args))
}

// shellJoin joins name and args into a single shell command string with each
// element single-quote-escaped, suitable for passing to ssh Session.Output.
func shellJoin(name string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	escape := func(s string) string {
		return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	}
	parts = append(parts, escape(name))
	for _, a := range args {
		parts = append(parts, escape(a))
	}
	return strings.Join(parts, " ")
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
		rel := strings.TrimPrefix(walker.Path(), srcPath)
		rel = strings.TrimPrefix(rel, "/")
		target := filepath.Join(dest, filepath.FromSlash(rel))
		if walker.Stat().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		} else if err := s.downloadFile(walker.Path(), target); err != nil {
			return err
		}
	}
	if err := walker.Err(); err != nil {
		return err
	}
	return nil
}

func (s *SFTP) Upload(localSrc, destDir string) error {
	info, err := os.Lstat(localSrc)
	if err != nil {
		return err
	}
	remoteDest := path.Join(destDir, filepath.Base(localSrc))
	if !info.IsDir() {
		return s.uploadFile(localSrc, remoteDest)
	}
	return filepath.Walk(localSrc, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(localSrc, p)
		if err != nil {
			return err
		}
		target := path.Join(remoteDest, filepath.ToSlash(rel))
		if fi.IsDir() {
			return s.client.MkdirAll(target)
		}
		return s.uploadFile(p, target)
	})
}

func (s *SFTP) uploadFile(localSrc, remoteDest string) error {
	if err := s.client.MkdirAll(path.Dir(remoteDest)); err != nil {
		return err
	}
	in, err := os.Open(localSrc)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := s.client.Create(remoteDest)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}

func (s *SFTP) Rename(oldPath, newName string) error {
	if strings.ContainsAny(newName, "/\\") || newName == "." || newName == ".." || newName == "" {
		return fmt.Errorf("invalid name: %q", newName)
	}
	return s.client.Rename(oldPath, path.Join(path.Dir(oldPath), newName))
}

func (s *SFTP) Remove(p string) error {
	return s.client.RemoveAll(p)
}

func (s *SFTP) Mkdir(p string) error {
	return s.client.Mkdir(p)
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
