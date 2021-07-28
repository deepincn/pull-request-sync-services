package github

import (
	"fmt"
	"os/exec"
	"strconv"

	"github.com/colorful-fullstack/PRTools/Controller"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type PushTask struct {
	manager *Manager
	number  int
	repo    string
}

func (t *PushTask) Name() string {
	return t.repo
}

func (t *PushTask) DoTask() error {
	result, err := t.manager.db.Find(t.repo, t.number)

	if err != nil {
		logrus.Error(err)
		return err
	}

	_number := fmt.Sprintf("%v", t.number)

	// add remote
	addRemote := exec.Command("git",
		"remote",
		"add",
		_number,
		fmt.Sprintf("https://%s:%s@github.com/%s/%s",
			*t.manager.conf.Auth.Github.User,
			*t.manager.conf.Auth.Github.Password,
			result.Sender.Login,
			t.repo,
		))
	addRemote.Dir = *t.manager.conf.RepoDir + t.repo

	// fetch remote
	refresh := exec.Command("git", "fetch", _number)
	refresh.Dir = *t.manager.conf.RepoDir + t.repo

	// switch branch
	// 切换到sender的分支
	switchBranch := exec.Command("git", "checkout", "-b", "tmp_"+_number, result.Head.Label)
	switchBranch.Dir = *t.manager.conf.RepoDir + t.repo

	// push remote
	push := exec.Command("git", "push", _number, "HEAD:"+result.Head.Ref, "-f")
	push.Dir = *t.manager.conf.RepoDir + t.repo

	// remove remote
	remove := exec.Command("git", "remote", "remove", _number)
	remove.Dir = *t.manager.conf.RepoDir + t.repo

	var list []*exec.Cmd
	list = append(list, addRemote, refresh, switchBranch, push, remove)
	return runCmdList(list)
}

func (m *Manager) MergeHandle(c *gin.Context) {
	number, err := strconv.Atoi(c.Param("number"))
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
