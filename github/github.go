package github

import (
	"io/ioutil"
	"net/http"

	"github.com/colorful-fullstack/PRTools/config"
	"github.com/google/go-github/v35/github"
	"github.com/sirupsen/logrus"
)

// Manager is github module manager
type Manager struct {
	conf *config.Yaml;
}

// New creates
func New(conf *config.Yaml) *Manager {
	return &Manager{
		conf: conf,
	}
}

// WebhookHandle init
func (m *Manager) WebhookHandle(rw http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var event interface{}

	payload, err := github.ValidatePayload(r, []byte(""))
	if err != nil {
		logrus.Errorf("validate payload failed: %v", err)
		return
	}

	event, err = github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		logrus.Errorf("parse webhook failed: %v", err)
		return
	}

	if err != nil {
		body, _ := ioutil.ReadAll(r.Body)
		logrus.Errorf("request body: %v", string(body))

		rw.WriteHeader(400)
		rw.Write([]byte(err.Error()))
		return
	}

	switch event := event.(type) {
	case *github.IssueEvent:
		logrus.Infof("IssueEvent: %v", *event.ID)
		break
	case *github.PullRequestEvent:
		logrus.Infof("PullRequestEvent: %v", *event.Number)
		m.pullrequestHandler(event)
		break
	case *github.PushEvent:
		logrus.Infof("PushEvent: %v", *event.PushID)
		break
	}
}
