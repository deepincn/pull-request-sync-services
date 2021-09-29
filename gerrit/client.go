package gerrit

import (
	"context"

	"github.com/deepincn/pull-request-sync-services/config"
	"golang.org/x/build/gerrit"
)

type Client struct {
	conf *config.Yaml
	gerrit *gerrit.Client
}

func NewClient(conf *config.Yaml) *Client {
	return &Client {
		conf: conf,
		gerrit: gerrit.NewClient(*conf.GerritServer, gerrit.BasicAuth(*conf.Auth.Gerrit.User, *conf.Auth.Gerrit.Password)),
	}
}

func (m *Client) Find(changeid string) (int, error) {
	ctx:= context.Background()
	detail, err := m.gerrit.GetChangeDetail(ctx, changeid, gerrit.QueryChangesOpt{})

	if err != nil {
		return -1, err
	}

	return detail.ChangeNumber, nil
}
