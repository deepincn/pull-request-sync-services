package database

import (
	"errors"

	"github.com/sirupsen/logrus"
	"gitlabwh.uniontech.com/zhangdingyuan/pull-request-sync-services/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Repo struct {
	Name  string
	Title string
	Body  string
}

type Head struct {
	Ref   string
	Label string
}

type Sender struct {
	Login string
	Author string
	Email string
}

type Base struct {
	Sha string
}

type Gerrit struct {
	ID int
	ChangeID string
}

type Github struct {
	ID int
}

type PullRequestModel struct {
	gorm.Model
	Github   Github `gorm:"embedded;embeddedPrefix:Github_"`
	Gerrit   Gerrit `gorm:"embedded;embeddedPrefix:Gerrit_"`
	Repo     Repo   `gorm:"embedded;embeddedPrefix:Repo_"`
	Head     Head   `gorm:"embedded;embeddedPrefix:Head_"`
	Sender   Sender `gorm:"embedded;embeddedPrefix:Sender_"`
	Base     Base   `gorm:"embedded;embeddedPrefix:Base_"`
}

type Find struct {
	Name string
	Github Github
	Gerrit Gerrit
}

type DataBase struct {
	db *gorm.DB
}

func NewDataBase(yaml *config.Yaml) *DataBase {
	db, err := gorm.Open(sqlite.Open(*yaml.Database.FileName), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	db.AutoMigrate(&PullRequestModel{})

	return &DataBase{
		db: db,
	}
}

func (db *DataBase) Create(record *PullRequestModel) error {
	result := db.db.Create(record)

	return result.Error
}

func (db *DataBase) Update(record *PullRequestModel) error {
	var value PullRequestModel
	result := db.db.Limit(1).Where(&PullRequestModel{
		Github: Github{
			ID: record.Github.ID,
		},
		Repo: Repo{
			Name: record.Repo.Name,
		},
		Gerrit: Gerrit{
			ID: record.Gerrit.ID,
			ChangeID: record.Gerrit.ChangeID,
		},
	}).First(&value)

	if result.RowsAffected == 0 {
		return errors.New("No record")
	}

	logrus.Debug(db.db.Model(&result).Updates(record))

	return nil
}

func (db *DataBase) FindByID(repo string, id int) (*PullRequestModel, error) {
	var value PullRequestModel
	result := db.db.Limit(1).Where(&PullRequestModel{
		Github: Github{
			ID: id,
		},
		Repo: Repo{
			Name: repo,
		},
	}).First(&value)

	if result.RowsAffected == 0 {
		return &PullRequestModel{}, errors.New("No record")
	}

	return &value, nil
}

func (db *DataBase) Find(find Find) (*PullRequestModel, error) {
	var value PullRequestModel
	result := db.db.Limit(1).Where(&PullRequestModel{
		Github: Github{
			ID: find.Github.ID,
		},
		Repo: Repo{
			Name: find.Name,
		},
		Gerrit: Gerrit{
			ID: find.Gerrit.ID,
			ChangeID: find.Gerrit.ChangeID,
		},
	}).First(&value)

	if result.RowsAffected == 0 {
		return &PullRequestModel{}, errors.New("No record")
	}

	return &value, nil
}
