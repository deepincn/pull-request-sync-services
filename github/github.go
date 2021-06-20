package github

import (
	"github.com/colorful-fullstack/PRTools/Controller"
	"github.com/colorful-fullstack/PRTools/config"
	"github.com/colorful-fullstack/PRTools/database"
	"github.com/google/go-github/v35/github"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
)

// Manager is github module manager
type Manager struct {
	conf *config.Yaml
	db   *database.DataBase
	taskChannel *chan Controller.Job
}

// New creates
func New(conf *config.Yaml, db *database.DataBase) *Manager {
	return &Manager{
		conf: conf,
		db:   db,
		taskChannel: Controller.InitPool(20, 5),
	}
}

// WebhookHandle init
func (m *Manager) WebhookHandle(rw http.ResponseWriter, r *http.Request) {
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

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logrus.Errorf("request body: %v", string(body))

		rw.WriteHeader(400)
		result, err := rw.Write([]byte(err.Error()))
		if err != nil {
			logrus.Errorf("rw write: %v", result)
		}
		return
	}

	switch event := event.(type) {
	case *github.IssueEvent:
		logrus.Infof("IssueEvent: %v", *event.ID)
		break
	case *github.PullRequestEvent:
		go func() {
			logrus.Infof("PullRequestEvent: %v", *event.Number)
			task := &PRTask {
				event: event,
				manager: m,
			}
			*m.taskChannel <- Controller.Job {
				Task: task,
			}
		}()
		break
	case *github.IssueCommentEvent:
		go func() {
			logrus.Infof("CommentEvent: %v", event.GetComment())
			task := &CommentTask{
				event: event,
				manager: m,
			}
			*m.taskChannel <- Controller.Job {
				Task: task,
			}
		}()
		break
	case *github.PushEvent:
		logrus.Infof("PushEvent: %v", *event.PushID)
		break
	}
}
