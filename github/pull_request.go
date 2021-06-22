package github

import (
	"bytes"
	"fmt"
	"github.com/colorful-fullstack/PRTools/database"
	"os"
	"os/exec"
	"strconv"

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
	event   *github.PullRequestEvent
	manager *Manager
}

func (t *PRTask) Name() string {
	return t.event.GetRepo().GetName()
}

func (t *PRTask) DoTask() error {
	t.pullRequestHandler(t.event)
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

// initRepo
func (this *PRTask) clone(repo *github.Repository) error {
	if _, err := os.Stat(*this.manager.conf.RepoDir + repo.GetName()); !os.IsNotExist(err) {
		return nil
	}

	var list []*exec.Cmd
	init := exec.Command("git", "clone", *this.manager.conf.Gerrit+repo.GetName())
	init.Dir = *this.manager.conf.RepoDir

	remote := exec.Command("git", "remote", "add", "github", "https://github.com/linuxdeepin/"+repo.GetName())
	remote.Dir = *this.manager.conf.RepoDir + repo.GetName()

	fetch := exec.Command("git", "fetch", "--all", "--tags")
	fetch.Dir = *this.manager.conf.RepoDir + repo.GetName()

	list = append(list, init, remote, fetch)

	return runCmdList(list)
}

// fetch
func (this *PRTask) fetch(repo *github.Repository, event *github.PullRequestEvent) error {
	var list []*exec.Cmd

	master := exec.Command("git", "checkout", "master", "-f")
	master.Dir = *this.manager.conf.RepoDir + repo.GetName()

	runSingleCmd(master)

	reset := exec.Command("git", "branch", "-D", fmt.Sprintf("dev_%v %v",
		strconv.Itoa(event.GetNumber()),
		strconv.Itoa(event.GetNumber())))
	reset.Dir = *this.manager.conf.RepoDir + repo.GetName()

	runSingleCmd(reset)

	refreshAll := exec.Command("git", "fetch", "--all")
	refreshAll.Dir = *this.manager.conf.RepoDir + repo.GetName()

	fetch := exec.Command("git", "fetch", "github", "pull/"+strconv.Itoa(event.GetNumber())+"/head:dev_"+strconv.Itoa(event.GetNumber()))
	fetch.Dir = *this.manager.conf.RepoDir + repo.GetName()

	list = append(list, refreshAll, fetch)

	return runCmdList(list)
}

// checkout
func (this *PRTask) checkout(repo *github.Repository, event *github.PullRequestEvent) error {
	var list []*exec.Cmd
	master := exec.Command("git", "checkout", "master", "-f")
	master.Dir = *this.manager.conf.RepoDir + repo.GetName()

	checkout := exec.Command("git", "checkout", "-B", strconv.Itoa(event.GetNumber()))
	checkout.Dir = *this.manager.conf.RepoDir + repo.GetName()
	list = append(list, master, checkout)

	return runCmdList(list)
}

// merge
func (this *PRTask) merge(repo *github.Repository, event *github.PullRequestEvent) error {
	var list []*exec.Cmd

	var msg string
	msg += "feat: " + event.PullRequest.GetTitle() + "\n\n"
	msg += event.PullRequest.GetBody() + "\n\n"
	msg += "Log:\n"
	msg += fmt.Sprintf("Issue: #%v\n", event.PullRequest.GetNumber()) // 记录提交来源，如果只有一个提交，也应当创建合并提交

	// TODO: check change id in database
	result, err := this.manager.db.Find(repo.GetName(), event.GetNumber())
	var changeId string
	if err != nil {
		changeId = generateChangeId()
		err := this.manager.db.Create(&database.PullRequestModel{
			Number:   event.GetNumber(),
			ChangeId: changeId,
			Repo: database.Repo{
				Name: event.Repo.GetName(),
				CloneUrl: event.Repo.GetCloneURL(),
			},
			Head: database.Head{
				Label: event.PullRequest.Head.GetLabel(),
				Ref: event.PullRequest.Head.GetRef(),
			},
		})
		if err != nil {
			return err
		}
	} else {
		changeId = result.ChangeId
	}

	logrus.Debugf("Change Id: %s", changeId)

	msg += "Change-Id: I" + changeId + "\n"

	squash := exec.Command("git", "merge", "dev_"+strconv.Itoa(event.GetNumber()), "--squash")
	squash.Dir = *this.manager.conf.RepoDir + repo.GetName()

	add := exec.Command("git", "add", ".")
	add.Dir = *this.manager.conf.RepoDir + repo.GetName()

	commit := exec.Command("git", "commit", "-m", msg)
	commit.Dir = *this.manager.conf.RepoDir + repo.GetName()

	list = append(list, squash, add, commit)

	err = runCmdList(list)

	if err != nil {
		reset := exec.Command("git", "merge", "--abort")
		reset.Dir = *this.manager.conf.RepoDir + repo.GetName()
		runSingleCmd(reset)
	}

	return err
}

func (this *PRTask) review(repo *github.Repository, event *github.PullRequestEvent) error {
	review := exec.Command("git", "review", "master", "-r", "origin")
	review.Dir = *this.manager.conf.RepoDir + repo.GetName()

	return runSingleCmd(review)
}

// pullRequestHandler is
func (this *PRTask) pullRequestHandler(event *github.PullRequestEvent) error {
	var err error
	for {
		if err = this.clone(event.Repo); err != nil {
			logrus.Errorf("clone: %v", err)
			break
		}

		if err = this.fetch(event.Repo, event); err != nil {
			logrus.Errorf("fetch: %v", err)
			break
		}

		if err = this.checkout(event.Repo, event); err != nil {
			logrus.Errorf("checkout: %v", err)
			break
		}

		if err = this.merge(event.Repo, event); err != nil {
			logrus.Errorf("merge: %v", err)
			break
		}

		if err = this.review(event.Repo, event); err != nil {
			logrus.Errorf("review: %v", err)
			break
		}
		break
	}

	return err
}
