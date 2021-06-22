package github

import (
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"os/exec"
	"strconv"
)

/*
由于gerrit不支持多个commit提交，所以github模块会在gerrit上产生一个合并压缩的提交，
为了在合并时让github上的状态能正确，例如正确显示合并的状态，需要在gerrit上合并的时候，先
调用本系统的合并，将合并推送到作者的pr分支，当gerrit上合并以后，同步到github上时即可自动合并。
 */

func (m *Manager) MergeHandle(c *gin.Context) {
	repo := c.Param("repo")
	_number := c.Param("number")
	number, err := strconv.Atoi(_number)

	if err != nil {
		logrus.Error("number not a int")
		return
	}

	result, err := m.db.Find(repo, number)

	if err != nil {
		logrus.Error(err)
		return
	}

	// add remote
	addRemote := exec.Command("git", "remote", "add", result.Repo.Name, result.Repo.CloneUrl)
	addRemote.Dir = *m.conf.RepoDir + repo
	runSingleCmd(addRemote)

	// fetch remote
	refresh := exec.Command("git", "fetch", result.Repo.Name)
	refresh.Dir = *m.conf.RepoDir + repo
	runSingleCmd(refresh)

	// switch branch
	// 切换到sender的分支
	switchBranch := exec.Command("git", "checkout", "-b", "tmp_" + _number, result.Head.Label)
	switchBranch.Dir = *m.conf.RepoDir + repo
	runSingleCmd(refresh)

	// push remote
	push := exec.Command("git", "push", result.Repo.Name, "HEAD:" + result.Head.Ref, "-f")
	push.Dir = *m.conf.RepoDir + repo
	runSingleCmd(push)
}