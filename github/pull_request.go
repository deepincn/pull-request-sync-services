package github

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/deepincn/pull-request-sync-services/database"
	"github.com/deepincn/pull-request-sync-services/gerrit"
	"github.com/deepincn/pull-request-sync-services/tools"

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
	return t.pullRequestHandler()
}

// initRepo
func (this *PRTask) clone() error {
	if _, err := os.Stat(this.Path()); !os.IsNotExist(err) {
		return nil
	}

	var list []*tools.Command

	init := &tools.Command{
		Program: "git",
		Args:    []string{"clone", *this.manager.conf.Gerrit + this.Name()},
		Dir:     *this.manager.conf.RepoDir,
		Timeout: 3600,
	}

	remote := &tools.Command{
		Program: "git",
		Args:    []string{"remote", "add", "github", "https://github.com/linuxdeepin/" + this.Name()},
		Dir:     this.Path(),
		Timeout: 3600,
	}

	fetch := &tools.Command{
		Program: "git",
		Args:    []string{"fetch", "--all"},
		Dir:     this.Path(),
		Timeout: 3600,
	}

	list = append(list, init, remote, fetch)

	return tools.RunCmdList(list)
}

// reset
func (this *PRTask) reset() error {
	tools.RunSingleCmd(&tools.Command{
		Program: "git",
		Args:    []string{"rebase", "--abort"},
		Dir: this.Path(),
		Timeout: 3600,
	})

	tools.RunSingleCmd(&tools.Command{
		Program: "git",
		Args:    []string{"checkout", "--track", "origin/" + this.Model.Base.Ref},
		Dir:     this.Path(),
		Timeout: 3600,
	})

	tools.RunSingleCmd(&tools.Command{
		Program: "git",
		Args:    []string{"checkout", this.Model.Base.Ref, "-f"},
		Dir:     this.Path(),
		Timeout: 3600,
	})

	// e.g. git branch -D 387 patch_387
	// 删除本地的pr对应分支
	var number = strconv.Itoa(this.Model.Github.ID)
	tools.RunSingleCmd(&tools.Command{
		Program: "git",
		Args: []string{"branch", "-D", fmt.Sprintf("%v",
			number,
		)},
		Dir:     this.Path(),
		Timeout: 3600,
	})

	//return tools.RunSingleCmd(reset)
	return nil
}

// rebase current branch
func (this *PRTask) rebase() error {
	logrus.Info("[rebase]...")
	if err := tools.RunSingleCmd(&tools.Command{
		Program: "git",
		Args:    []string{"rebase", this.Model.Base.Ref},
		Dir:     this.Path(),
		Timeout: 3600,
	}); err != nil {
		tools.RunSingleCmd(&tools.Command{
			Program: "git",
			Args:    []string{"rebase", "--abort"},
			Dir: this.Path(),
			Timeout: 3600,
		})
		return err
	}

	return nil
}

// fetch
func (this *PRTask) fetch() error {
	this.reset()

	var number = strconv.Itoa(this.Model.Github.ID)

	// 下载最新的分支
	if err := tools.RunSingleCmd(&tools.Command{
		Program: "git",
		Args: []string{"fetch", "github", fmt.Sprintf("pull/%v/head:%v",
			number,
			number,
		)},
		Dir:     this.Path(),
		Timeout: 3600,
	}); err != nil {
		return err
	}

	tools.RunSingleCmd(&tools.Command{
		Program: "git",
		Args:    []string{"checkout", number},
		Dir:     this.Path(),
		Timeout: 3600,
	})

	r, _ := git.PlainOpen(this.Path())
	ref, _ := r.Head()
	cIter, _ := r.Log(&git.LogOptions{From: ref.Hash()})
	commit, _ := cIter.Next()
	this.Model.Sender.Author = commit.Author.Name
	this.Model.Sender.Email = commit.Author.Email

	if err := this.rebase(); err != nil {
		return err
	}

	return tools.RunSingleCmd(&tools.Command{
		Program: "bash",
		Args:    []string{"-c", fmt.Sprintf("git diff %v > %v", this.Model.Base.Sha, this.diffFile)},
		Dir:     this.Path(),
		Timeout: 3600,
	})
}

// checkout
func (this *PRTask) checkout() error {
	tools.RunSingleCmd(&tools.Command{
		Program: "git",
		Args:    []string{"checkout", this.Model.Base.Ref, "-f"},
		Dir:     this.Path(),
		Timeout: 3600,
	})

	// 创建一个对应pr的patch分支
	tools.RunSingleCmd(&tools.Command{
		Program: "git",
		Args: []string{"branch", "-D", fmt.Sprintf("patch_%v",
			strconv.Itoa(this.Model.Github.ID),
		)},
		Dir:     this.Path(),
		Timeout: 3600,
	})

	return tools.RunSingleCmd(&tools.Command{
		Program: "git",
		Args: []string{"checkout", "-b", fmt.Sprintf("patch_%v",
			strconv.Itoa(this.Model.Github.ID),
		)},
		Dir:     this.Path(),
		Timeout: 3600,
	})
}

// merge
func (this *PRTask) patch() error {
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

	patch := &tools.Command{
		Program: "git",
		Args:    []string{"apply", this.diffFile},
		Dir:     this.Path(),
		Timeout: 3600,
	}

	add := &tools.Command{
		Program: "git",
		Args:    []string{"add", "."},
		Dir:     this.Path(),
		Timeout: 3600,
	}

	logrus.Debug(this.Model.Sender.Author, this.Model.Sender.Email)

	commit := &tools.Command{
		Program: "git",
		Args: []string{"commit", "-m", msg, fmt.Sprintf("--author=\"%v <%v>\"",
			this.Model.Sender.Author,
			this.Model.Sender.Email,
		)},
		Dir:     this.Path(),
		Timeout: 3600,
	}

	var list []*tools.Command
	list = append(list, patch, add, commit)

	err = tools.RunCmdList(list)

	return err
}

func (this *PRTask) review() error {
	return tools.RunSingleCmd(&tools.Command{
		Program: "git",
		Args:    []string{"review", this.Model.Head.Ref, "-r", "origin"},
		Dir:     this.Path(),
		Timeout: 3600,
	})
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
