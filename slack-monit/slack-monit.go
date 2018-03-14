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

	"github.com/ivanpesin/slack-relay/slack"
	"gopkg.in/yaml.v2"
)

const appVersion = "1.1"

type options struct {
	Relay   string `yaml:"relay"`
	File    string
	PostURL string `yaml:"post_url"`
	Channel string `yaml:"channel"`

	Color string

	Debug bool `yaml:"debug"`
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
	if config.Debug {
		log.Print(s)
	}
}

func readConfigFile() {
	if _, err := os.Stat(config.File); err == nil {
		buf, err := ioutil.ReadFile(config.File)
		if err != nil {
			log.Fatalf("Unable to read config file: %v", err)
		}

		var cf options
		err = yaml.Unmarshal(buf, &cf)
		if err != nil {
			log.Fatalf("Unable to parse config file: %v", err)
		}

		debug(fmt.Sprintf("Configuration from file %s: %+v\n", config.File, cf))
		// set if not overriden by cli flag
		if config.PostURL == "" {
			config.PostURL = cf.PostURL
		}
		if config.Channel == "" {
			config.Channel = cf.Channel
		}
		if config.Relay == "" {
			config.Relay = cf.Relay
		}
	}
	if config.Channel == "" {
		config.Channel = "#random"
	}
}

func readMonitData() {
	// Get monit information
	monit.service = os.Getenv("MONIT_SERVICE")
	monit.event = os.Getenv("MONIT_EVENT")
	monit.description = os.Getenv("MONIT_DESCRIPTION")
	monit.host = os.Getenv("MONIT_HOST")
	monit.date = os.Getenv("MONIT_DATE")

	// guess color for the message if not set
	if config.Color == "" {
		config.Color = "danger"
		if strings.Contains(monit.event, "succe") || strings.Contains(monit.event, "Exists") {
			config.Color = "good"
		}
	}

}

func formPayload() *slack.Message {
	// create payload for slack message
	payload := &slack.Message{}
	payload.Channel = config.Channel
	payload.Attachments = append(payload.Attachments, slack.Attachment{})
	payload.Attachments[0].Color = config.Color
	payload.Attachments[0].Fallback = fmt.Sprintf("%s: %s on %s\n%s", monit.service, monit.event, monit.host, monit.description)
	payload.Attachments[0].Text = fmt.Sprintf("`%s`: *%s*\n%s", monit.service, monit.event, monit.description)
	payload.Attachments[0].MrkdwnIn = []string{"text"}
	payload.Attachments[0].Fields = []slack.Field{
		slack.Field{Title: "Date", Value: monit.date, Short: true},
		slack.Field{Title: "Host", Value: monit.host, Short: true},
	}

	return payload
}

func sendSlackMessage(m *slack.Message) error {
	// create JSON
	buf, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to create JSON payload: %v", err)
	}

	// post to Slack
	debug(fmt.Sprintf("Sending to %s, payload:\n%s", config.PostURL, buf))
	resp, err := http.Post(config.PostURL, "application/json", bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("failed to send message to slack: %v", err)
	}

	// process response
	b, _ := ioutil.ReadAll(resp.Body)
	debug(fmt.Sprintf("Response received, status: %s\nBody:\n%s", resp.Status, b))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error status received: %v\nBody:\n%s", resp.StatusCode, b)
	}
	return nil
}

func init() {
	flag.StringVar(&config.File, "f", cfFile, "config file")
	flag.StringVar(&config.Channel, "channel", config.Channel, "channel to post to")
	flag.StringVar(&config.PostURL, "url", config.PostURL, "slack webhook")
	flag.StringVar(&config.Relay, "relay", config.Relay, "slack raw protocol relay")
	flag.BoolVar(&config.Debug, "d", false, "enable debug messages")
}

func main() {
	flag.Parse()
	readConfigFile()

	if config.PostURL == "" {
		log.Fatalf("Slack webhook POST URL is required.")
	}

	readMonitData()

	p := formPayload()
	if err := sendSlackMessage(p); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
