package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"

	"github.com/google/go-github/v35/github"
)

// 克隆github的仓库，添加gerrit仓库，创建一个merge提交，提交到gerrit.
// 数据库记录id的对应关系，当github分支更新时，重复该操作。
// body : {
//   repo,
//   event
// }

// Manager struct is a manager
type Manager struct {
	event *github.PullRequestEvent
}

// Init pull request
func (m *Manager) Init(event *github.PullRequestEvent) {
	m.event = event
}

type Command struct {
	Program  string
	Args     []string
	WorkPath string
}

// CloneRepo patch
func (m *Manager) CloneRepo() error {
	repoName := m.event.GetRepo().GetName()

	// clone base
	list := []Command{
		{
			Program:  "git",
			Args:     []string{"clone", "ssh://ut000063@gerrit.uniontech.com:29418/" + repoName},
			WorkPath: "/tmp/",
		},
		{
			Program:  "git",
			Args:     []string{"remote", "add", "github", "https://github.com/linuxdeepin/" + repoName},
			WorkPath: "/tmp/" + repoName,
		},
		{
			Program:  "git",
			Args:     []string{"fetch", "--all", "--tags"},
			WorkPath: "/tmp/" + repoName,
		},
	}

	for _, command := range list {
		cmd := exec.Cmd{
			Dir:  command.WorkPath,
			Path: command.Program,
			Args: command.Args,
		}

		err := cmd.Run()

		if err != nil {
			return err
		}
	}

	return nil
}

// push gerrit

// sync comment

func initRepo(event *github.PullRequestEvent) {
	repoManager := &Manager{}
	repoManager.Init(event)
}

func webhookHandle(rw http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	event := &github.PullRequestEvent{}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Println(err)
		return
	}
	json.Unmarshal([]byte(body), event)
	go initRepo(event)
}

func main() {
	http.HandleFunc("/", webhookHandle)
}
