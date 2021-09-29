package service

import (
	"context"
	"fmt"

	"github.com/deepincn/pull-request-sync-services/config"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

// Service is a struct
type Service struct {
	conf *config.Yaml
}

// 按项目保存
type Repo struct {
	Name string
	Issue *github.Issue
}

// NewService get a service
func NewService(conf *config.Yaml) *Service {
	return &Service{
		conf,
	}
}

func (m *Service) RefreshData() {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: *m.conf.Auth.Github.Token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 10},
	}
	// list all repositories for the authenticated user
    var allRepos []*github.Repository
	for {
		repos, resp, err := client.Repositories.ListByOrg(ctx, "linuxdeepin", opt)
		if err != nil {
			break
		}
		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	// 保存目前所有的仓库列表
	for _, repo := range allRepos {
		opt := &github.IssueListByRepoOptions{
			State: "all",
			ListOptions: github.ListOptions{PerPage: 10},
		}
		var allIssues []*github.Issue
		for {
			issues, resp, err := client.Issues.ListByRepo(ctx, "linuxdeepin", repo.GetName(), opt)
			if err != nil {
				break
			}
			allIssues = append(allIssues, issues...)
			if resp.NextPage == 0 {
				break
			}
			opt.Page = resp.NextPage
		}
		for _, issue := range allIssues {
			if issue.IsPullRequest() {
				continue
			}
			fmt.Printf("%s,%s,%d\n", repo.GetName(), issue.GetState(), issue.GetNumber())
		}
	}
}
