package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/satori/go.uuid"

	"github.com/Jeffail/gabs"
)

var listenSocket = flag.String("l", "0.0.0.0:8081", "socket to listen on")
var gatewayURL = flag.String("gw", "http://localhost:8080", "slack gateway URL")

func main() {
	log.Printf("Slack Relay URL: %v", *gatewayURL)
	log.Printf("Listening on   : %v", *listenSocket)

	l, err := net.Listen("tcp", *listenSocket)
	if err != nil {
		log.Fatalf("Error listening: %v", err)
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatalf("Error accepting: %v", err)
		}

		go handleRequest(conn)
	}

}

func handleRequest(conn net.Conn) {

	rid := uuid.NewV4().String()[:8]
	log.Printf("[%s] accepted: %v", rid, conn.RemoteAddr().String())
	defer conn.Close()

	r := gabs.New() // request
	a := gabs.New() // attachment if any

	text := ""
	pretext := ""
	var readin *string // pointer to text or pretext, depending on what we are reading
	readText := false
	asAttachment := false

	s := bufio.NewScanner(conn)
	for s.Scan() {
		if readText {
			// read until single dot on a line or end of stream
			if s.Text() == "." {
				readText = false
			} else {
				*readin = *readin + "\n" + s.Text()
			}
		} else {
			f := strings.Fields(s.Text())
			if len(f) < 1 {
				continue
			}
			switch {
			case strings.ToUpper(f[0]) == "CHANNEL":
				if len(f) < 2 {
					log.Printf("E: Invalid number of arguments: %v", s.Text())
					continue
				}
				r.SetP(f[1], "channel")
			case strings.ToUpper(f[0]) == "LEVEL":
				if len(f) < 2 {
					log.Printf("E: Invalid number of arguments: %v", s.Text())
					continue
				}
				asAttachment = true
				a.SetP(f[1], "color")
			case strings.ToUpper(f[0]) == "FIELD":
				if len(f) < 4 {
					log.Printf("E: Invalid number of arguments: %v", s.Text())
					continue
				}
				asAttachment = true
				field := gabs.New()

				field.SetP(f[1], "title")
				if strings.ToUpper(f[2]) == "SHORT" {
					field.SetP(true, "short")
				} else {
					field.SetP(false, "short")
				}
				pos := len(f[0] + " " + f[1] + " " + f[2] + " ")
				field.SetP(s.Text()[pos:], "value")

				if !a.Exists("fields") {
					a.Array("fields")
				}
				a.ArrayAppend(field.Data(), "fields")
			case strings.ToUpper(f[0]) == "TEXT":
				readin = &text
				readText = true
			case strings.ToUpper(f[0]) == "PRETEXT":
				asAttachment = true
				readin = &pretext
				readText = true
			}
			// read text after keyword if any
			if readText && len(f) > 1 {
				*readin = s.Text()[len(f[0])+1:]
			}

		}
	}

	if text == "" && pretext == "" {
		text = "<empty> (this is wrong)"
	}

	if asAttachment {
		if text != "" {
			a.SetP(text, "text")
		}
		if pretext != "" {
			a.SetP(pretext, "pretext")
		}
		a.Array("mrkdwn_in")
		a.ArrayAppend("text", "mrkdwn_in")
		a.ArrayAppend("pretext", "mrkdwn_in")
		r.Array("attachments")
		r.ArrayAppend(a.Data(), "attachments")
	} else {
		r.SetP(text, "text")
	}

	log.Printf("[%s] sending: %v", rid, r.String())

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Post(*gatewayURL, "application/json", strings.NewReader(r.String()))
	if err != nil {
		log.Printf("[%s] E: %v", rid, err)
		conn.Write([]byte("Failed: " + err.Error()))
		return
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		log.Printf("[%s] E: code=%d %s", rid, resp.StatusCode, string(body))
		conn.Write([]byte(fmt.Sprintf("Failed: code=%d %s", resp.StatusCode, string(body))))
	}

	log.Printf("[%s] resp: %v", rid, string(body))
	conn.Write(body)
	return
}
