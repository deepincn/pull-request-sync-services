package database

import (
	"errors"
	"github.com/colorful-fullstack/PRTools/config"
)

type PullRequest struct {
	Number   *int    `json:"Number"`
	ChangeId *string `json:"ChangeId"`
}

type DataBase struct {
	config *config.Yaml
	result struct {
		pullRequest []PullRequest
	}
}

func NewDataBase(config *config.Yaml) *DataBase {
	return &DataBase{
		config: config,
	}
}

func (m PullRequest) GetNumber() int {
	return *m.Number
}

func (m PullRequest) GetChangeId() string {
	return *m.ChangeId
}

func (m DataBase) SetChangeId(number int, changeId string) bool {
	for _, v := range m.result.pullRequest {
		if v.GetNumber() == number {
			v.ChangeId = &changeId
			break
		}
	}

	// TODO: save

	return true
}

func (m DataBase) GetChangeId(number int) (string, error) {
	for _, v := range m.result.pullRequest {
		if v.GetNumber() == number {
			return v.GetChangeId(), nil
		}
	}
	return "", errors.New("ChangeId not found")
}
