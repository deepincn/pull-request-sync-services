package github

import (
	"fmt"
	"io/ioutil"

	"github.com/gin-gonic/gin"

	"github.com/colorful-fullstack/PRTools/Controller"
	"github.com/colorful-fullstack/PRTools/config"
	"github.com/colorful-fullstack/PRTools/database"
	"github.com/google/go-github/v35/github"
	"github.com/sirupsen/logrus"
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
func (m *Manager) WebhookHandle(c *gin.Context) {
	var event interface{}

	payload, err := github.ValidatePayload(c.Request, []byte(""))
	if err != nil {
		logrus.Errorf("validate payload failed: %v", err)
		return
	}

	event, err = github.ParseWebHook(github.WebHookType(c.Request), payload)
	if err != nil {
		logrus.Errorf("parse webhook failed: %v", err)
		return
	}

	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		logrus.Errorf("request body: %v", string(body))

		c.Writer.WriteHeader(400)
		result, err := c.Writer.Write([]byte(err.Error()))
		if err != nil {
			logrus.Errorf("rw write: %v", result)
		}
		return
	}

	switch event := event.(type) {
	case *github.PingEvent:
		logrus.Infof("PingEvent: %v", event.GetHook().Events)
	case *github.IssueEvent:
		logrus.Infof("IssueEvent: %v", *event.ID)
	case *github.PullRequestEvent:
		go func() {
			logrus.Infof("PullRequestEvent: %v", *event.Number)
			task := &PRTask {
				event: event,
				manager: m,
				diffFile: fmt.Sprintf("/tmp/#%v.#%v.diff", event.Repo.GetName(), event.GetNumber()),
			}
			*m.taskChannel <- Controller.Job {
				Task: task,
			}
		}()
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
	case *github.PushEvent:
		logrus.Infof("PushEvent: %v", *event.PushID)
	}
}
