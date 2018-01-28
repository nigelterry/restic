package backend

import "github.com/restic/restic/internal/restic"

// PcloudLayout implements the default layout for the REST protocol.
type PcloudLayout struct {
	URL  string
	Path string
	Join func(...string) string
}

var pcloudLayoutPaths = defaultLayoutPaths

func (l *PcloudLayout) String() string {
	return "<PcloudLayout>"
}

// Name returns the name for this layout.
func (l *PcloudLayout) Name() string {
	return "rest"
}

// Dirname returns the directory path for a given file type and name.
func (l *PcloudLayout) Dirname(h restic.Handle) string {
	if h.Type == restic.ConfigFile {
		return l.URL + l.Join(l.Path, "/")
	}

	return l.URL + l.Join(l.Path, "/", pcloudLayoutPaths[h.Type]) + "/"
}

// Filename returns a path to a file, including its name.
func (l *PcloudLayout) Filename(h restic.Handle) string {
	name := h.Name

	if h.Type == restic.ConfigFile {
		name = "config"
	}

	return l.URL + l.Join(l.Path, "/", pcloudLayoutPaths[h.Type], name)
}

// Paths returns all directory names
func (l *PcloudLayout) Paths() (dirs []string) {
	for _, p := range pcloudLayoutPaths {
		dirs = append(dirs, l.URL+l.Join(l.Path, p))
	}
	return dirs
}

// Basedir returns the base dir name for files of type t.
func (l *PcloudLayout) Basedir(t restic.FileType) (dirname string, subdirs bool) {
	return l.URL + l.Join(l.Path, pcloudLayoutPaths[t]), false
}
