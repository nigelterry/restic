package pcloud

import (
	"strings"

	"github.com/restic/restic/internal/errors"
	"github.com/restic/restic/internal/options"
)

// Config holds all information needed to open a pcloud repository.
type Config struct {
	Path   string
	Layout string `option:"layout" help:"use this backend directory layout (default: auto-detect)"`
}

func init() {
	options.Register("pcloud", Config{})
}

// ParseConfig parses a pcloud backend config.
func ParseConfig(cfg string) (interface{}, error) {
	if !strings.HasPrefix(cfg, "pcloud:") {
		return nil, errors.New(`invalid format, prefix "pcloud" not found`)
	}

	return Config{Path: cfg[6:]}, nil
}
