package database

import (
	"errors"
	"github.com/colorful-fullstack/PRTools/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Repo struct {
	Name     string
	CloneUrl string
}

type Head struct {
	Ref   string
	Label string
}

type PullRequestModel struct {
	gorm.Model
	Number   int
	ChangeId string
	Repo     Repo `gorm:"embedded;embeddedPrefix:Repo_"`
	Head     Head `gorm:"embedded;embeddedPrefix:Head_"`
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

func (db *DataBase) Find(repo string, number int) (*PullRequestModel, error) {
	var value PullRequestModel
	result := db.db.Limit(1).Where(&PullRequestModel{
		Number: number,
		Repo: Repo{
			Name: repo,
		},
	}).First(&value)

	if result.RowsAffected == 0 {
		return &PullRequestModel{}, errors.New("No record")
	}

	return &value, nil
}
