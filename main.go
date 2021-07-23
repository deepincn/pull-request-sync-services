package main

import (
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/colorful-fullstack/PRTools/database"
	"github.com/colorful-fullstack/PRTools/service"

	"github.com/colorful-fullstack/PRTools/config"
	"github.com/colorful-fullstack/PRTools/github"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

func init() {
	lvl, ok := os.LookupEnv("LOG_LEVEL")
	// LOG_LEVEL not set, let's default to debug
	if !ok {
		lvl = "debug"
	}
	// parse string, this is built-in feature of logrus
	ll, err := logrus.ParseLevel(lvl)
	if err != nil {
		ll = logrus.DebugLevel
	}
	// set global log level
	logrus.SetLevel(ll)
}

func main() {
	conf := new(config.Yaml)
	yamlFile, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		logrus.Infof("yamlFile.Get err #%v ", err)
	}
	err = yaml.Unmarshal([]byte(yamlFile), conf)
	if err != nil {
		logrus.Fatalf("Unmarshal: %v", err)
	}
	service := service.NewService(conf)
	service.RefreshData()
	return
	db := database.NewDataBase(conf)

	githubManager := github.New(conf, db)

	router := gin.Default()
	router.POST("/merge/:repo/:number", githubManager.MergeHandle)
	router.POST("/webhook/github", githubManager.WebhookHandle)
	srv := &http.Server{
		Handler: router,
		Addr:    "127.0.0.1:3002",
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	logrus.Fatal(srv.ListenAndServe())
}
