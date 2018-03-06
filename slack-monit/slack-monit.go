package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

const appVersion = "1.0"

type options struct {
	file    string
	postURL string
	channel string

	color string

	debug bool
}

type slackMessage struct {
	Channel     string                              `json:"channel"`
	Fallback    string                              `json:"fallback"`
	Color       string                              `json:"color"`
	Text        string                              `json:"text"`
	MrkdwnIn    []string                            `json:"mrkdwn_in"`
	Attachments []map[string][]slackAttachmentField `json:"attachments"`
}

type slackAttachmentField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

var monit struct {
	service     string
	event       string
	description string
	host        string
	date        string
}

var config options
var cfFile = "/etc/slack-monit.conf"

func debug(s string) {
	if config.debug {
		log.Print(s)
	}
}

func readConfigFile() {
	if _, err := os.Stat(config.file); err == nil {
		buf, err := ioutil.ReadFile(config.file)
		if err != nil {
			log.Fatalf("Unable to read config file: %v", err)
		}

		var cf options
		err = yaml.Unmarshal(buf, &cf)
		if err != nil {
			log.Fatalf("Unable to parse config file: %v", err)
		}

		// set if not overriden by cli flag
		if config.postURL == "" {
			config.postURL = cf.postURL
		}
		if config.channel == "" {
			config.channel = cf.channel
		}

	}
}

func init() {
	flag.StringVar(&config.file, "f", cfFile, "config file")
	flag.StringVar(&config.channel, "channel", "#random", "channel to post to")
	flag.StringVar(&config.postURL, "url", config.postURL, "slack webhook")
	flag.BoolVar(&config.debug, "d", false, "enable debug messages")
}

func main() {
	flag.Parse()
	readConfigFile()

	monit.service = os.Getenv("MONIT_SERVICE")
	monit.event = os.Getenv("MONIT_EVENT")
	monit.description = os.Getenv("MONIT_DESCRIPTION")
	monit.host = os.Getenv("MONIT_HOST")
	monit.date = os.Getenv("MONIT_DATE")

	if config.color == "" {
		if strings.Contains(monit.event, "succe") || strings.Contains(monit.event, "Exists") {
			config.color = "good"
		} else {
			config.color = "danger"
		}
	}

	payload := &slackMessage{}
	payload.Channel = config.channel
	payload.Color = config.color
	payload.Fallback = fmt.Sprintf("%s: %s on %s\n%s", monit.service, monit.event, monit.host, monit.description)
	payload.Text = fmt.Sprintf("`%s`: *%s*\n%s", monit.service, monit.event, monit.description)
	payload.MrkdwnIn = []string{"text"}
	payload.Attachments = make([]map[string][]slackAttachmentField, 1)
	payload.Attachments[0] = make(map[string][]slackAttachmentField)
	payload.Attachments[0]["fields"] = []slackAttachmentField{
		slackAttachmentField{Title: "Date", Value: monit.date, Short: true},
		slackAttachmentField{Title: "Host", Value: monit.host, Short: true},
	}

	buf, err := json.MarshalIndent(&payload, "", "  ")
	if err != nil {
		log.Fatalf("Failed to create JSON payload: %v\n", err)
	}

	debug(fmt.Sprintf("Sending to %s, payload:\n%s", config.postURL, buf))

	resp, err := http.Post(config.postURL, "application/json", bytes.NewReader(buf))
	if err != nil {
		log.Fatalf("Failed to send message to slack: %v\n", err)
	}

	b, _ := ioutil.ReadAll(resp.Body)
	debug(fmt.Sprintf("Response received, status: %s\nBody:\n%s", resp.Status, b))

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Error status received: %v", resp.StatusCode)
	}
}
