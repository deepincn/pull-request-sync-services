package gerrit

import (
	"gitlabwh.uniontech.com/zhangdingyuan/pull-request-sync-services/config"
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
