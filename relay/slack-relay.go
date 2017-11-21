package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/satori/go.uuid"

	"github.com/Jeffail/gabs"
)

var slackURL = os.Getenv("SLACK_GW_URL")
var listenSocket = flag.String("l", "0.0.0.0:8080", "socket to listen on")

func sendToSlack(payload string) (string, error) {

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Post(
		slackURL,
		"application/json",
		strings.NewReader(payload))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("%s - %s", resp.Status, err.Error())
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s - %s", resp.Status, string(body))
	}

	return string(body), nil
}

func requestError(w http.ResponseWriter, id string, code int, m string) {
	log.Printf("[%s] %d - %s", id, code, m)

	w.WriteHeader(code)
	io.WriteString(w, m)
	return
}

func handler(w http.ResponseWriter, r *http.Request) {

	rid := uuid.NewV4().String()[:8]

	b, _ := ioutil.ReadAll(r.Body)
	parsed, err := gabs.ParseJSON(b)
	if err != nil {
		log.Printf("[%s] body: %s", rid, string(b))
		requestError(w, rid, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("[%s] sending: %s", rid, parsed.String())

	var resp string
	resp, err = sendToSlack(parsed.String())
	if err != nil {
		requestError(w, rid, http.StatusBadGateway, "slack resp: "+err.Error())
		return
	}

	log.Printf("[%s] %s", rid, resp)
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, resp)
	return
}

func main() {

	log.Printf("Slack URL: %v", slackURL)
	log.Printf("Listening on: %v", *listenSocket)

	// subscribe to SIGINT signals
	stopChan := make(chan os.Signal)
	signal.Notify(stopChan, os.Interrupt)

	log.Printf("CTRL+C or SIGINT to stop service ...")

	mux := http.NewServeMux()
	mux.HandleFunc("/", handler)
	srv := &http.Server{Addr: *listenSocket, Handler: mux}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("E: listen: %s\n", err)
			time.Sleep(time.Second)
		}
	}()

	<-stopChan
	// shut down gracefully, but wait no longer than 5 seconds before halting
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)

	log.Println("Server gracefully stopped")
}
