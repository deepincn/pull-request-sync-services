package gerrit

import (
	"github.com/colorful-fullstack/PRTools/config"
	"golang.org/x/build/gerrit"
)

type Client struct {
	conf *config.Yaml
	gerrit *gerrit.Client
}

func NewClient(conf *config.Yaml) *Client {
	return &Client {
		conf: conf,
		gerrit: gerrit.NewClient("https://gerrit.uniontech.com", gerrit.BasicAuth(*conf.Auth.Gerrit.User, *conf.Auth.Gerrit.Password)),
	}
}
