package main

import (
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/deepincn/pull-request-sync-services/database"

	"github.com/deepincn/pull-request-sync-services/config"
	"github.com/deepincn/pull-request-sync-services/github"
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
	debug := os.Getenv("DEBUG_MODE")
	conf := new(config.Yaml)

	var yamlFile []byte
	var err error
	if debug == "TRUE" {
		yamlFile, err = ioutil.ReadFile("config.yaml")
	} else {
		yamlFile, err = ioutil.ReadFile("/etc/sync/config.yaml")
	}
	if err != nil {
		logrus.Infof("yamlFile.Get err #%v ", err)
	}
	err = yaml.Unmarshal([]byte(yamlFile), conf)
	if err != nil {
		logrus.Fatalf("Unmarshal: %v", err)
	}
	// service := service.NewService(conf)
	// service.RefreshData()
	// return
	db := database.NewDataBase(conf)

	githubManager := github.New(conf, db)

	router := gin.Default()
	router.POST("/github/:repo/:id", githubManager.SyncHandle)
	router.POST("/webhook/github", githubManager.WebhookHandle)
	router.PUT("/gerrit/:repo/:id", githubManager.MergeHandle)
	srv := &http.Server{
		Handler: router,
		Addr:    "0.0.0.0:3002",
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	logrus.Fatal(srv.ListenAndServe())
}
