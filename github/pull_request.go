package github

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"

	"github.com/colorful-fullstack/PRTools/database"
	"golang.org/x/oauth2"

	"github.com/google/go-github/v35/github"
	"github.com/sirupsen/logrus"
)

/*
	git fetch pull/<number>/head:<author>_<commit_hash>
	git checkout -b merge_<author>_<commit_hash>
	git merge --no-ff <author>_<commit_hash>

	generate change-id to commit
	save branch and change-id to database

	-------------------------------------------------

	git checkout master
	git branch -D merge_<author>_<commit_hash>

	init step again
	and read change-id from database

	--------------------------------------------------

	in the end

	git review master -r origin
*/

/*
1. 用户fork了repo，并创建了pr，请求合并分支里的提交
2. gerrit只接受单一commit，但是允许提交merge提交
3. 同步工具会创建以pr号为分支名的本地分支
4. 同步工具拉去对应的pull/head,并创建merge提交，根据规范填充commit msg
5. 同步工具review提交
6. 当评论提交到gerrit上后，gerrit插件应该提供review时的分支，用于确定github上的pr号
*/

func DownloadFile(filepath string, url string) error {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

func generateChangeId() string {
	output := &bytes.Buffer{}
	c1 := exec.Command("whoami; hostname; date;")
	c2 := exec.Command("git", "hash-object", "--stdin")
	c2.Stdin, _ = c1.StdoutPipe()
	c2.Stdout = output
	_ = c2.Start()
	_ = c1.Run()
	_ = c2.Wait()
	return output.String()
}

type PRTask struct {
	event    *github.PullRequestEvent
	manager  *Manager
	diffFile string
}

func (t *PRTask) Name() string {
	return t.event.GetRepo().GetName()
}

func (t *PRTask) DoTask() error {
	t.pullRequestHandler()
	return nil
}

func runSingleCmd(command *exec.Cmd) error {
	stdout, err := command.StdoutPipe()
	command.Stderr = command.Stdout
	if err = command.Start(); err != nil {
		return err
	}
	for {
		tmp := make([]byte, 1024)
		_, err := stdout.Read(tmp)
		logrus.Debug(string(tmp))
		if err != nil {
			break
		}
	}
	return command.Wait()
}

func runCmdList(list []*exec.Cmd) error {
	for _, command := range list {
		if err := runSingleCmd(command); err != nil {
			return err
		}
	}

	return nil
}

func (this *PRTask) downloadDiff() error {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: *this.manager.conf.Auth.Github.Token},
	)
	client := github.NewClient(oauth2.NewClient(ctx, ts))
	pr, _, err := client.PullRequests.Get(ctx, "linuxdeepin", this.event.Repo.GetName(), this.event.GetNumber())
	if err != nil {
		return err
	}
	return DownloadFile(this.diffFile, pr.GetDiffURL())
}

// initRepo
func (this *PRTask) clone() error {
	if _, err := os.Stat(*this.manager.conf.RepoDir + this.event.Repo.GetName()); !os.IsNotExist(err) {
		return nil
	}

	var list []*exec.Cmd
	init := exec.Command("git", "clone", *this.manager.conf.Gerrit+this.event.Repo.GetName())
	init.Dir = *this.manager.conf.RepoDir

	remote := exec.Command("git", "remote", "add", "github", "https://github.com/linuxdeepin/"+this.event.Repo.GetName())
	remote.Dir = *this.manager.conf.RepoDir + this.event.Repo.GetName()

	fetch := exec.Command("git", "fetch", "--all", "--tags")
	fetch.Dir = *this.manager.conf.RepoDir + this.event.Repo.GetName()

	list = append(list, init, remote, fetch)

	return runCmdList(list)
}

// fetch
func (this *PRTask) fetch() error {
	// var list []*exec.Cmd

	master := exec.Command("git", "checkout", "master", "-f")
	master.Dir = *this.manager.conf.RepoDir + this.event.Repo.GetName()

	if err := runSingleCmd(master); err != nil {
		return err
	}

	// e.g. git branch -D dev_1
	reset := exec.Command("git", "branch", "-D", fmt.Sprintf("%v",
		strconv.Itoa(this.event.GetNumber())))
	reset.Dir = *this.manager.conf.RepoDir + this.event.Repo.GetName()

	if err := runSingleCmd(reset); err != nil {
		return err
	}

	return nil
}

// checkout
func (this *PRTask) checkout() error {
	var list []*exec.Cmd
	master := exec.Command("git", "checkout", "master", "-f")
	master.Dir = *this.manager.conf.RepoDir + this.event.Repo.GetName()

	checkout := exec.Command("git", "checkout", "-B", strconv.Itoa(this.event.GetNumber()))
	checkout.Dir = *this.manager.conf.RepoDir + this.event.Repo.GetName()
	list = append(list, master, checkout)

	return runCmdList(list)
}

// merge
func (this *PRTask) patch() error {
	var list []*exec.Cmd

	var msg string
	msg += "feat: " + this.event.PullRequest.GetTitle() + "\n\n"
	msg += this.event.PullRequest.GetBody() + "\n\n"
	msg += "Log:\n"
	msg += fmt.Sprintf("Issue: #%v\n", this.event.PullRequest.GetNumber())

	// TODO: check change id in database
	result, err := this.manager.db.Find(this.event.Repo.GetName(), this.event.GetNumber())
	var changeID string
	if err != nil {
		changeID = generateChangeId()
		err := this.manager.db.Create(&database.PullRequestModel{
			Number:   this.event.GetNumber(),
			ChangeId: changeID,
			Repo: database.Repo{
				Name:     this.event.Repo.GetName(),
				CloneUrl: this.event.Repo.GetCloneURL(),
			},
			Head: database.Head{
				Label: this.event.PullRequest.Head.GetLabel(),
				Ref:   this.event.PullRequest.Head.GetRef(),
			},
			Sender: database.Sender{
				Login: this.event.Sender.GetLogin(),
			},
		})
		if err != nil {
			return err
		}
	} else {
		changeID = result.ChangeId
	}

	logrus.Debugf("Change Id: %s", changeID)

	msg += "Change-Id: I" + changeID + "\n"

	patch := exec.Command("git", "apply", this.diffFile)
	patch.Dir = *this.manager.conf.RepoDir + this.event.Repo.GetName()

	add := exec.Command("git", "add", ".")
	add.Dir = *this.manager.conf.RepoDir + this.event.Repo.GetName()

	commit := exec.Command("git", "commit", "-m", msg)
	commit.Dir = *this.manager.conf.RepoDir + this.event.Repo.GetName()

	list = append(list, patch, add, commit)

	err = runCmdList(list)

	return err
}

func (this *PRTask) review() error {
	review := exec.Command("git", "review", "master", "-r", "origin")
	review.Dir = *this.manager.conf.RepoDir + this.event.Repo.GetName()

	return runSingleCmd(review)
}

// pullRequestHandler is
// - 下载diff文件
// - 初始化本地git
// - 创建 commit
// - 提交 gerrit（生成changeid）
func (this *PRTask) pullRequestHandler() error {
	var err error
	for {
		if err = this.downloadDiff(); err != nil {
			logrus.Errorf("download diff: %v", err)
			break
		}
		if err = this.clone(); err != nil {
			logrus.Errorf("clone: %v", err)
			break
		}

		if err = this.fetch(); err != nil {
			logrus.Errorf("fetch: %v", err)
			break
		}

		if err = this.checkout(); err != nil {
			logrus.Errorf("checkout: %v", err)
			break
		}

		if err = this.patch(); err != nil {
			logrus.Errorf("patch: %v", err)
			logrus.Errorf("reset commit: %v", this.fetch())
			break
		}

		if err = this.review(); err != nil {
			logrus.Errorf("review: %v", err)
			logrus.Errorf("reset commit: %v", this.fetch())
			break
		}
		break
	}

	return err
}
