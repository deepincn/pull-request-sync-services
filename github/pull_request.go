package github

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"gitlabwh.uniontech.com/zhangdingyuan/pull-request-sync-services/database"
	"gitlabwh.uniontech.com/zhangdingyuan/pull-request-sync-services/gerrit"

	"github.com/go-git/go-git/v5"
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
	c1 := exec.Command("date")
	c2 := exec.Command("git", "hash-object", "--stdin")
	c2.Stdin, _ = c1.StdoutPipe()
	c2.Stdout = output
	_ = c2.Start()
	_ = c1.Run()
	_ = c2.Wait()
	return output.String()
}

type PRTask struct {
	manager  *Manager
	diffFile string
	Model    database.PullRequestModel
}

func (t *PRTask) Name() string {
	return t.Model.Repo.Name
}

func (t *PRTask) Path() string {
	return *t.manager.conf.RepoDir + t.Name()
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

// initRepo
func (this *PRTask) clone() error {
	if _, err := os.Stat(this.Path()); !os.IsNotExist(err) {
		return nil
	}

	var list []*exec.Cmd
	init := exec.Command("git", "clone", *this.manager.conf.Gerrit+this.Name())
	init.Dir = *this.manager.conf.RepoDir

	remote := exec.Command("git", "remote", "add", "github", "https://github.com/linuxdeepin/"+this.Name())
	remote.Dir = this.Path()

	fetch := exec.Command("git", "fetch", "--all", "--tags")
	fetch.Dir = this.Path()

	list = append(list, init, remote, fetch)

	return runCmdList(list)
}

// reset
func (this *PRTask) reset() error {
	master := exec.Command("git", "checkout", "master", "-f")
	master.Dir = this.Path()

	if err := runSingleCmd(master); err != nil {
		return err
	}

	// e.g. git branch -D 387 patch_387
	// 删除本地的pr对应分支
	var number = strconv.Itoa(this.Model.Github.ID)
	reset := exec.Command("git", "branch", "-D", fmt.Sprintf("%v",
		number,
	))
	reset.Dir = this.Path()

	return runSingleCmd(reset)
}

// rebase current branch
func (this *PRTask) rebase() error {
	rebase := exec.Command("git", "rebase", "master")
	rebase.Dir = this.Path()
	if err := runSingleCmd(rebase); err != nil {
		restore := exec.Command("git", "rebase", "--abort")
		restore.Dir = this.Path()
		runSingleCmd(restore)
		return err
	}

	return nil
}

// fetch
func (this *PRTask) fetch() error {
	this.reset()

	var number = strconv.Itoa(this.Model.Github.ID)

	// 下载最新的分支
	fetch := exec.Command("git", "fetch", "github", fmt.Sprintf("pull/%v/head:%v",
		number,
		number,
	))
	fetch.Dir = this.Path()

	if err := runSingleCmd(fetch); err != nil {
		return err
	}

	checkout2pr := exec.Command("git", "checkout", number)
	checkout2pr.Dir = this.Path()

	runSingleCmd(checkout2pr)

	r, _ := git.PlainOpen(this.Path())
	ref, _ := r.Head()
	cIter, _ := r.Log(&git.LogOptions{From: ref.Hash()})
	commit, _ := cIter.Next()
	this.Model.Sender.Author = commit.Author.Name
	this.Model.Sender.Email = commit.Author.Email

	if err := this.rebase(); err != nil {
		return err
	}

	diff := exec.Command("bash", "-c", fmt.Sprintf("git diff %v > %v", this.Model.Base.Sha, this.diffFile))
	diff.Dir = this.Path()

	return runSingleCmd(diff)
}

// checkout
func (this *PRTask) checkout() error {
	master := exec.Command("git", "checkout", "master", "-f")
	master.Dir = this.Path()
	runSingleCmd(master)

	// 创建一个对应pr的patch分支
	reset := exec.Command("git", "branch", "-D", fmt.Sprintf("patch_%v",
		strconv.Itoa(this.Model.Github.ID),
	))
	reset.Dir = this.Path()

	runSingleCmd(reset)

	checkout := exec.Command("git", "checkout", "-b", fmt.Sprintf("patch_%v",
		strconv.Itoa(this.Model.Github.ID),
	))
	checkout.Dir = this.Path()
	return runSingleCmd(checkout)
}

// merge
func (this *PRTask) patch() error {
	var list []*exec.Cmd

	var msg string
	msg += "feat: " + this.Model.Repo.Title + "\n\n"
	msg += this.Model.Repo.Body + "\n\n"
	msg += "Log:\n"
	msg += fmt.Sprintf("Issue: #%v\n", this.Model.Github.ID)

	// TODO: check change id in database
	find := database.Find{
		Name: this.Name(),
		Github: database.Github{
			ID: this.Model.Github.ID,
		},
	}
	result, err := this.manager.db.Find(find)
	var changeID string
	if err != nil {
		changeID = generateChangeId()
		record := this.Model
		record.Gerrit.ChangeID = changeID
		// NOTE: 不要忘记还没有更新 gerrit id
		err := this.manager.db.Create(&record)
		if err != nil {
			return err
		}
	} else {
		changeID = result.Gerrit.ChangeID
	}

	logrus.Debugf("Change Id: %s", changeID)

	msg += "Change-Id: I" + changeID + "\n"

	patch := exec.Command("git", "apply", this.diffFile)
	patch.Dir = this.Path()

	add := exec.Command("git", "add", ".")
	add.Dir = this.Path()

	commit := exec.Command("git", "commit", "-m", msg, fmt.Sprintf("--author=\"%v <%v>\"",
		this.Model.Sender.Author,
		this.Model.Sender.Email,
	))
	logrus.Debug(this.Model.Sender.Author, this.Model.Sender.Email)
	commit.Dir = this.Path()

	list = append(list, patch, add, commit)

	err = runCmdList(list)

	return err
}

func (this *PRTask) review() error {
	review := exec.Command("git", "review", "master", "-r", "origin")
	review.Dir = this.Path()

	return runSingleCmd(review)
}

func (this *PRTask) updateGerrit() error {
	gerrit := gerrit.NewClient(this.manager.conf)
	number, err := gerrit.Find(this.Model.Gerrit.ChangeID)
	if err != nil {
		return err
	}
	record := this.Model
	record.Gerrit.ID = number

	// update to database
	return this.manager.db.Update(&record)
}

// pullRequestHandler is
// - 下载diff文件
// - 初始化本地git
// - 创建 commit
// - 提交 gerrit（生成changeid）
func (this *PRTask) pullRequestHandler() error {
	var err error
	for {
		if err = this.clone(); err != nil {
			logrus.Errorf("clone: %v", err)
			break
		}

		if err = this.fetch(); err != nil {
			logrus.Errorf("fetch: %v", err)
			logrus.Errorf("reset fetch: %v", this.reset())
			break
		}

		if err = this.checkout(); err != nil {
			logrus.Errorf("checkout: %v", err)
			break
		}

		if err = this.patch(); err != nil {
			logrus.Errorf("patch: %v", err)
			logrus.Errorf("reset commit: %v", this.reset())
			break
		}

		if err = this.review(); err != nil {
			logrus.Errorf("review: %v", err)
			logrus.Errorf("reset commit: %v", this.reset())
			break
		}

		if err = this.updateGerrit(); err != nil {
			logrus.Errorf("updateGerrit: %v", err)
			//TODO: 这里需要考虑更新数据库失败后的操作
			break
		}
		break
	}

	return err
}
