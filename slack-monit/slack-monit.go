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
const defaultConfigFile = "/etc/slack-monit.conf"

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

// shows a message if in debug mode
func debug(s string) {
	if config.Debug {
		log.Print(s)
	}
}

// returns the location of configuration file
func configFile() string {
	// highest priority if specified on command line
	if len(os.Args) > 2 {
		for i, a := range os.Args {
			if a == "-c" && i+1 < len(os.Args) {
				return os.Args[i+1]
			}
		}
	}
	// check environment variable
	s := os.Getenv("SLACK_MONIT_CONF")
	if s != "" {
		return s
	}
	// return the default
	return defaultConfigFile
}

// load configuration from a file
func readConfigFile() {
	if _, err := os.Stat(config.File); err == nil {
		buf, err := ioutil.ReadFile(config.File)
		if err != nil {
			log.Fatalf("Unable to read config file: %v", err)
		}

		err = yaml.Unmarshal(buf, &config)
		if err != nil {
			log.Fatalf("Unable to parse config file: %v", err)
		}

		debug(fmt.Sprintf("Configuration from file %s: %+v\n", config.File, config))
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
	flag.StringVar(&config.File, "c", defaultConfigFile, "config file")
	flag.StringVar(&config.Channel, "channel", "#random", "slack channel to post to")
	flag.StringVar(&config.PostURL, "url", "", "slack webhook")
	flag.StringVar(&config.Relay, "relay", "", "slack raw protocol relay")
	flag.BoolVar(&config.Debug, "d", false, "enable debug messages")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}
}

func main() {

	readConfigFile()
	flag.Parse()

	if config.PostURL == "" {
		log.Fatalf("Slack webhook POST URL is required.")
	}

	readMonitData()

	p := formPayload()
	if err := sendSlackMessage(p); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
