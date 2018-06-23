package main

import (
	"context"
	"encoding/json"
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
)

var config struct {
	PostURL string `yaml:"post_url"`
	LSock   string `yaml:"listen"`
}

func sendToSlack(payload string) (string, error) {

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Post(
		config.PostURL,
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

	rid := uuid.Must(uuid.NewV4()).String()[:8]

	b, _ := ioutil.ReadAll(r.Body)
	if !json.Valid(b) {
		log.Printf("[%s] body: %s", rid, string(b))
		requestError(w, rid, http.StatusBadRequest, "Unable to parse JSON")
		return
	}

	log.Printf("[%s] sending: %s", rid, b)

	var resp string
	resp, err := sendToSlack(string(b))
	if err != nil {
		requestError(w, rid, http.StatusBadGateway, "slack resp: "+err.Error())
		return
	}

	log.Printf("[%s] %s", rid, resp)
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, resp)
	return
}

func init() {
	config.PostURL = os.Getenv("SLACK_GW_URL")
	flag.StringVar(&config.LSock, "l", "0.0.0.0:8080", "socket to listen on")

	flag.Parse()
}

func main() {

	log.Printf("Slack URL: %v", config.PostURL)
	log.Printf("Listening on: %v", config.LSock)

	// subscribe to SIGINT signals
	stopChan := make(chan os.Signal)
	signal.Notify(stopChan, os.Interrupt)

	log.Printf("CTRL+C or SIGINT to stop service ...")

	mux := http.NewServeMux()
	mux.HandleFunc("/", handler)
	srv := &http.Server{Addr: config.LSock, Handler: mux}

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

	if ctx.Err() == context.DeadlineExceeded {
		log.Println("Server shutdown timeout, forced termination")
	} else {
		log.Println("Server stopped")
	}
}
