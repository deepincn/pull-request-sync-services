package github

import (
	"context"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"

	"github.com/deepincn/pull-request-sync-services/Controller"
	"github.com/deepincn/pull-request-sync-services/config"
	"github.com/deepincn/pull-request-sync-services/database"
	"github.com/google/go-github/v35/github"
	"github.com/sirupsen/logrus"
)

// Manager is github module manager
type Manager struct {
	conf        *config.Yaml
	db          *database.DataBase
	taskChannel *chan Controller.Job
}

// New creates
func New(conf *config.Yaml, db *database.DataBase) *Manager {
	return &Manager{
		conf:        conf,
		db:          db,
		taskChannel: Controller.InitPool(20, 5),
	}
}

func (m *Manager) SyncHandle(c *gin.Context) {
	repo := c.Param("repo")
	id, err := strconv.Atoi(c.Param("id"))
	forceUpdate := c.DefaultQuery("force", "false")

	if err != nil {
		logrus.Error("number not a int")
		return
	}

	// find a record
	var record *database.PullRequestModel
	record, err = m.db.FindByID(repo, id)
	if forceUpdate == "true" || err != nil {
		var task *PRTask
		task, err = m.SyncPR(repo, id, true)
		if err != nil {
			logrus.Errorf("Failed to sync pull request: ", err)
			return
		}
		err = task.DoTask()
		if err != nil {
			logrus.Errorf("Failed to do sync pull request: ", err)
			c.JSON(500, "")
			return
		}
	}

	record, err = m.db.FindByID(repo, id)
	type Result struct {
		Repo        string `json:"repo,omitempty"`
		PullRequest struct {
			Number   int    `json:"number,omitempty"`
			Hash     string `json:"hash,omitempty"`
			RepoHash string `json:"repo_hash,omitempty"`
		} `json:"pull_request,omitempty"`
		Author struct {
			UserName string `json:"username,omitempty"`
			Email    string `json:"email,omitempty"`
		} `json:"author,omitempty"`
		Gerrit struct {
			Number   int    `json:"number,omitempty"`
			ChangeID string `json:"changeid,omitempty"`
		} `json:"gerrit,omitempty"`
	}
	c.JSON(200, &Result{
		Repo: repo,
		PullRequest: struct {
			Number   int    `json:"number,omitempty"`
			Hash     string `json:"hash,omitempty"`
			RepoHash string `json:"repo_hash,omitempty"`
		}{
			Number:   id,
			Hash:     "",
			RepoHash: "",
		},
		Author: struct {
			UserName string `json:"username,omitempty"`
			Email    string `json:"email,omitempty"`
		}{
			UserName: record.Sender.Author,
			Email:    record.Sender.Email,
		},
		Gerrit: struct {
			Number   int    `json:"number,omitempty"`
			ChangeID string `json:"changeid,omitempty"`
		}{
			Number:   record.Gerrit.ID,
			ChangeID: record.Gerrit.ChangeID,
		},
	})
}

func (m *Manager) SyncPR(repo string, id int, forceUpdate bool) (*PRTask, error) {
	// init client
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: *m.conf.Auth.Github.Token},
	)

	client := github.NewClient(oauth2.NewClient(ctx, ts))

	pr, _, err := client.PullRequests.Get(ctx, "linuxdeepin", repo, id)

	if err != nil {
		return nil, err
	}

	branch, _, err := client.Repositories.GetBranch(ctx, "linuxdeepin", repo, pr.Base.GetRef())

	if err != nil {
		return nil, err
	}

	return &PRTask{
		Model: database.PullRequestModel{
			Github: database.Github{
				ID: id,
			},
			Repo: database.Repo{
				Name:  repo,
				Title: pr.GetTitle(),
				Body:  pr.GetBody(),
				Sha: strings.Trim(branch.GetCommit().GetSHA(), "origin/"),
			},
			Head: database.Head{
				Label: pr.Head.GetLabel(),
				Ref:   pr.Head.GetRef(),
			},
			Sender: database.Sender{
				Login:  pr.User.GetLogin(),
			},
			Base: database.Base{
				Sha: pr.Base.GetSHA(),
				Ref: pr.Base.GetRef(),
			},
		},
		manager:  m,
		diffFile: fmt.Sprintf("/tmp/#%v.#%v.diff", repo, id),
		forceUpdate: forceUpdate,
	}, nil
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
		logrus.Infof("PullRequestEvent: %v", *event.Number)
		go func() {
			task, err := m.SyncPR(event.Repo.GetName(), event.GetNumber(), true)
			if err != nil {
				logrus.Errorf("Error syncing pull request: %v", err)
				return
			}
			*m.taskChannel <- Controller.Job{
				Task: task,
			}
		}()
	case *github.IssueCommentEvent:
		go func() {
			logrus.Infof("CommentEvent: %v", event.GetComment())
			task := &CommentTask{
				event:   event,
				manager: m,
			}
			*m.taskChannel <- Controller.Job{
				Task: task,
			}
		}()
	case *github.PushEvent:
		logrus.Infof("PushEvent: %v", *event.PushID)
	}
}
