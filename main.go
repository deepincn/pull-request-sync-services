package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/colorful-fullstack/PRTools/config"
	"github.com/colorful-fullstack/PRTools/github"
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

	logrus.Debug(conf)

	githubManager := github.New(conf)
	http.HandleFunc("/", githubManager.WebhookHandle)
	logrus.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", 3002), nil))
}
