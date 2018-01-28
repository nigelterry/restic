package pcloud

import (
	"strings"
	"log"

	"github.com/restic/restic/internal/errors"
	"github.com/restic/restic/internal/options"
	"net/url"
)

// Config holds all information needed to open a pcloud repository.
type Config struct {
	URL    *url.URL
	Path   string
	Layout string `option:"layout" help:"use this backend directory layout (default: auto-detect)"`

	UserName string
	Password string
	AuthToken string

	Connections uint `option:"connections" help:"set a limit for the number of concurrent connections (default: 20)"`
}

// NewConfig returns a new Config with the default values filled in.
func NewConfig() Config {
	u, err := url.Parse("https://api.pcloud.com")
	if err != nil {
		log.Fatal(err)
	}
	return Config{
		Connections: 20,
		URL: u,

	}
}

func init() {
	options.Register("pcloud", Config{})
}

// ParseConfig parses a pcloud backend config.
func ParseConfig(s string) (interface{}, error) {
	if !strings.HasPrefix(s, "pcloud:") {
		return nil, errors.New(`invalid format, prefix "pcloud" not found`)
	}

	// strip prefix "pcloud:"
	s = s[7:]

	// use the first entry of the path as the path, UserName and Password name and the
	// remainder as prefix
	data := strings.SplitN(s, ":", 3)
	if len(data) < 3 {
		return nil, errors.New("pcloud: invalid format: needs Username:Password:Path")
	}

	cfg := NewConfig()
	cfg.UserName = data[0]
	cfg.Password = data[1]
	cfg.Path = data[2]

	return cfg, nil
}