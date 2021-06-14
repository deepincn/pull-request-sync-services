package github

import (
	"bytes"
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

// initRepo
func (m *Manager) clone(repo *github.Repository) error {
	if _, err := os.Stat(m.conf.RepoDir + repo.GetName()); !os.IsNotExist(err) {
		return nil
	}

	var list []*exec.Cmd
	init := exec.Command("git", "clone", m.conf.Gerrit+repo.GetName())
	init.Dir = m.conf.RepoDir

	remote := exec.Command("git", "remote", "add", "github", "https://github.com/linuxdeepin/"+repo.GetName())
	remote.Dir = m.conf.RepoDir + repo.GetName()

	fetch := exec.Command("git", "fetch", "--all", "--tags")
	fetch.Dir = m.conf.RepoDir + repo.GetName()

	list = append(list, init, remote, fetch)

	for _, command := range list {
		err := command.Run()
		if err != nil {
			return err
		}
	}

	return nil
}

// fetch
func (m *Manager) fetch(repo *github.Repository, event *github.PullRequestEvent) error {
	var list []*exec.Cmd
	fetch := exec.Command("git", "fetch", "github", "pull/"+strconv.Itoa(event.GetNumber())+"/head:"+strconv.Itoa(event.GetNumber()))
	fetch.Dir = m.conf.RepoDir + repo.GetName()

	list = append(list, fetch)

	for _, command := range list {
		if err := command.Run(); err != nil {
			return err
		}
	}

	return nil
}

// checkout
func (m *Manager) checkout(repo *github.Repository, event *github.PullRequestEvent) error {
	var list []*exec.Cmd
	checkout := exec.Command("git", "checkout", "master")
	checkout.Dir = m.conf.RepoDir + repo.GetName()
	list = append(list, checkout)

	for _, command := range list {
		if err := command.Run(); err != nil {
			return err
		}
	}

	return nil
}

// merge
func (m *Manager) merge(repo *github.Repository, event *github.PullRequestEvent) error {
	var list []*exec.Cmd

	var msg string
	msg += "feat: " + event.PullRequest.GetTitle() + "\n\n"
	msg += event.PullRequest.GetBody() + "\n\n"
	msg += "Log:\n"

	// TODO: check change id in database
	msg += "Change-Id: I" + generateChangeId() + "\n"

	merge := exec.Command("git", "merge", strconv.Itoa(event.GetNumber()), "-m", msg)
	merge.Dir = m.conf.RepoDir + repo.GetName()
	list = append(list, merge)

	for _, command := range list {
		if err := command.Run(); err != nil {
			return err
		}
	}

	return nil
}

// pullrequestHandler is
func (m *Manager) pullrequestHandler(event *github.PullRequestEvent) {
	var err error
	if err = m.clone(event.Repo); err != nil {
		logrus.Errorf("clone: %v", err)
		return
	}

	if err = m.fetch(event.Repo, event); err != nil {
		logrus.Errorf("fetch: %v", err)
		return
	}

	if err = m.checkout(event.Repo, event); err != nil {
		logrus.Errorf("checkout: %v", err)
		return
	}

	if err = m.merge(event.Repo, event); err != nil {
		logrus.Errorf("merge: %v", err)
		return
	}
}
