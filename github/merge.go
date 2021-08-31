package github

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/go-github/github"
	"github.com/sirupsen/logrus"
	"gitlabwh.uniontech.com/zhangdingyuan/pull-request-sync-services/Controller"
	"gitlabwh.uniontech.com/zhangdingyuan/pull-request-sync-services/database"
	"golang.org/x/oauth2"
)

type PushTask struct {
	manager *Manager
	number  int
	repo    string
}

func (t *PushTask) Name() string {
	return t.repo
}

// 先合并原本的pr，再强制推送一次gerrit的数据
func (t *PushTask) DoTask() error {
	find := database.Find{
		Name: t.repo,
		Gerrit: database.Gerrit{
			ID: t.number,
		},
	}
	_, err := t.manager.db.Find(find)

	if err != nil {
		logrus.Error(err)
		return err
	}

	_number := fmt.Sprintf("%v", t.number)

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: *t.manager.conf.Auth.Github.Token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	options := &github.PullRequestOptions{
		MergeMethod: "merge",
	}
	_, _, err = client.PullRequests.Merge(ctx, "linuxdeepin", t.repo, t.number, "", options)

	if err != nil {
		logrus.Error(err)
		return err
	}

	// push gerrit branch to overwrite github pr
	checkout := []string{"checkout", "master", "-f"}
	fetch := []string{"fetch", "--all"}
	push := []string{"push", "origin", "master", "-f"}
	remove := []string{"branch", "-D", _number, fmt.Sprintf("patch_%v", _number)}

	// remove remote
	var list [][]string
	list = append(list, checkout, fetch, push, remove)

	for _, v := range list {
		b := exec.Command("git", v...)
		b.Dir = *t.manager.conf.RepoDir + t.repo

		if err := b.Run(); err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) MergeHandle(c *gin.Context) {
	number, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		logrus.Error("number not a int")
		return
	}
	task := &PushTask{
		manager: m,
		repo:    c.Param("repo"),
		number:  number,
	}
	*m.taskChannel <- Controller.Job{
		Task: task,
	}
}
