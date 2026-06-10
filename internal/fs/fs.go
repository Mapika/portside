// Package fs abstracts the filesystems portside can browse: the local disk
// and remote hosts over SFTP.
package fs

import (
	"sort"
	"strings"
)

type Entry struct {
	Name  string
	Path  string // absolute path on the backend
	IsDir bool
}

type Filesystem interface {
	// Name identifies the backend: "local" or the SSH host alias.
	Name() string
	Home() (string, error)
	List(path string) ([]Entry, error)
	// Download copies the file or directory at srcPath into the local
	// directory destDir (recursively for directories).
	Download(srcPath, destDir string) error
	// Upload copies the local file or directory at localSrc into destDir on
	// the backend (recursively for directories).
	Upload(localSrc, destDir string) error
	// Rename renames the file or directory at oldPath to newName (a bare
	// name, not a path; the parent directory stays the same).
	Rename(oldPath, newName string) error
	// Remove deletes the file or directory at path recursively.
	Remove(path string) error
	// Mkdir creates a new directory at path.
	Mkdir(path string) error
}

// Sort orders entries directories-first, then case-insensitively by name.
func Sort(entries []Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
}
