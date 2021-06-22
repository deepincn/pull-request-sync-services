package github

import "github.com/google/go-github/v35/github"

type CommentTask struct {
	event *github.IssueCommentEvent
	manager *Manager
}

func (t *CommentTask) Name() string {
	return t.event.GetRepo().GetName()
}

func (t *CommentTask) DoTask() error {
	return t.pushComment()
}

func (c *CommentTask) pushComment() error {

	return nil
}