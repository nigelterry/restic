package pcloud

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/nigelterry/restic/internal/errors"
	"github.com/nigelterry/restic/internal/restic"

	"github.com/nigelterry/restic/internal/backend"
	"github.com/nigelterry/restic/internal/debug"
	"github.com/nigelterry/restic/internal/fs"
	"net/http"
)

// Pcloud is a backend in a pcloud directory.
type Pcloud struct {
	Config
	backend.Layout
}

// ensure statically that *Pcloud implements restic.Backend.
var _ restic.Backend = &Pcloud{}

const defaultLayout = "default"

// dirExists returns true if the name exists and is a directory.
func dirExists(name string) bool {
	f, err := fs.Open(name)
	if err != nil {
		return false
	}

	fi, err := f.Stat()
	if err != nil {
		return false
	}

	if err = f.Close(); err != nil {
		return false
	}

	return fi.IsDir()
}

// Open opens the pcloud backend as specified by config.
func Open(cfg Config, rt http.RoundTripper) (*Pcloud, error) {
	debug.Log("open pcloud backend at %v (layout %q)", cfg.Path, cfg.Layout)
	l, err := backend.ParseLayout(&backend.PcloudFilesystem{}, cfg.Layout, defaultLayout, cfg.Path)
	if err != nil {
		return nil, err
	}

	return &Pcloud{Config: cfg, Layout: l}, nil
}

// Create creates all the necessary files and directories for a new pcloud
// backend at dir. Afterwards a new config blob should be created.
func Create(cfg Config, rt http.RoundTripper) (*Pcloud, error) {
	debug.Log("create pcloud backend at %v (layout %q)", cfg.Path, cfg.Layout)

	l, err := backend.ParseLayout(&backend.PcloudFilesystem{}, cfg.Layout, defaultLayout, cfg.Path)
	if err != nil {
		return nil, err
	}

	be := &Pcloud{
		Config: cfg,
		Layout: l,
	}

	// test if config file already exists
	_, err = fs.Lstat(be.Filename(restic.Handle{Type: restic.ConfigFile}))
	if err == nil {
		return nil, errors.New("config file already exists")
	}

	// create paths for data and refs
	for _, d := range be.Paths() {
		err := fs.MkdirAll(d, backend.Modes.Dir)
		if err != nil {
			return nil, errors.Wrap(err, "MkdirAll")
		}
	}

	return be, nil
}

// Location returns this backend's location (the directory name).
func (b *Pcloud) Location() string {
	return b.Path
}

// IsNotExist returns true if the error is caused by a non existing file.
func (b *Pcloud) IsNotExist(err error) bool {
	return os.IsNotExist(errors.Cause(err))
}

// Save stores data in the backend at the handle.
func (b *Pcloud) Save(ctx context.Context, h restic.Handle, rd io.Reader) error {
	debug.Log("Save %v", h)
	if err := h.Valid(); err != nil {
		return err
	}

	filename := b.Filename(h)

	// create new file
	f, err := fs.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, backend.Modes.File)

	if b.IsNotExist(err) {
		debug.Log("error %v: creating dir", err)

		// error is caused by a missing directory, try to create it
		mkdirErr := os.MkdirAll(filepath.Dir(filename), backend.Modes.Dir)
		if mkdirErr != nil {
			debug.Log("error creating dir %v: %v", filepath.Dir(filename), mkdirErr)
		} else {
			// try again
			f, err = fs.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, backend.Modes.File)
		}
	}

	if err != nil {
		return errors.Wrap(err, "OpenFile")
	}

	// save data, then sync
	_, err = io.Copy(f, rd)
	if err != nil {
		_ = f.Close()
		return errors.Wrap(err, "Write")
	}

	if err = f.Sync(); err != nil {
		_ = f.Close()
		return errors.Wrap(err, "Sync")
	}

	err = f.Close()
	if err != nil {
		return errors.Wrap(err, "Close")
	}

	return setNewFileMode(filename, backend.Modes.File)
}

// Load returns a reader that yields the contents of the file at h at the
// given offset. If length is nonzero, only a portion of the file is
// returned. rd must be closed after use.
func (b *Pcloud) Load(ctx context.Context, h restic.Handle, length int, offset int64) (io.ReadCloser, error) {
	debug.Log("Load %v, length %v, offset %v", h, length, offset)
	if err := h.Valid(); err != nil {
		return nil, err
	}

	if offset < 0 {
		return nil, errors.New("offset is negative")
	}

	f, err := fs.Open(b.Filename(h))
	if err != nil {
		return nil, err
	}

	if offset > 0 {
		_, err = f.Seek(offset, 0)
		if err != nil {
			_ = f.Close()
			return nil, err
		}
	}

	if length > 0 {
		return backend.LimitReadCloser(f, int64(length)), nil
	}

	return f, nil
}

// Stat returns information about a blob.
func (b *Pcloud) Stat(ctx context.Context, h restic.Handle) (restic.FileInfo, error) {
	debug.Log("Stat %v", h)
	if err := h.Valid(); err != nil {
		return restic.FileInfo{}, err
	}

	fi, err := fs.Stat(b.Filename(h))
	if err != nil {
		return restic.FileInfo{}, errors.Wrap(err, "Stat")
	}

	return restic.FileInfo{Size: fi.Size(), Name: h.Name}, nil
}

// Test returns true if a blob of the given type and name exists in the backend.
func (b *Pcloud) Test(ctx context.Context, h restic.Handle) (bool, error) {
	debug.Log("Test %v", h)
	_, err := fs.Stat(b.Filename(h))
	if err != nil {
		if os.IsNotExist(errors.Cause(err)) {
			return false, nil
		}
		return false, errors.Wrap(err, "Stat")
	}

	return true, nil
}

// Remove removes the blob with the given name and type.
func (b *Pcloud) Remove(ctx context.Context, h restic.Handle) error {
	debug.Log("Remove %v", h)
	fn := b.Filename(h)

	// reset read-only flag
	err := fs.Chmod(fn, 0666)
	if err != nil {
		return errors.Wrap(err, "Chmod")
	}

	return fs.Remove(fn)
}

func isFile(fi os.FileInfo) bool {
	return fi.Mode()&(os.ModeType|os.ModeCharDevice) == 0
}

// List runs fn for each file in the backend which has the type t. When an
// error occurs (or fn returns an error), List stops and returns it.
func (b *Pcloud) List(ctx context.Context, t restic.FileType, fn func(restic.FileInfo) error) error {
	debug.Log("List %v", t)

	basedir, subdirs := b.Basedir(t)
	return fs.Walk(basedir, func(path string, fi os.FileInfo, err error) error {
		debug.Log("walk on %v\n", path)
		if err != nil {
			return err
		}

		if path == basedir {
			return nil
		}

		if !isFile(fi) {
			return nil
		}

		if fi.IsDir() && !subdirs {
			return filepath.SkipDir
		}

		debug.Log("send %v\n", filepath.Base(path))

		rfi := restic.FileInfo{
			Name: filepath.Base(path),
			Size: fi.Size(),
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		err = fn(rfi)
		if err != nil {
			return err
		}

		return ctx.Err()
	})
}

// Delete removes the repository and all files.
func (b *Pcloud) Delete(ctx context.Context) error {
	debug.Log("Delete()")
	return fs.RemoveAll(b.Path)
}

// Close closes all open files.
func (b *Pcloud) Close() error {
	debug.Log("Close()")
	// this does not need to do anything, all open files are closed within the
	// same function.
	return nil
}
