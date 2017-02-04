package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type Attachment struct {
	Text     string `json:"text"`
	ImageURL string `json:"image_url"`
}

type Payload struct {
	ResponseType string       `json:"response_type"`
	Attachments  []Attachment `json:"attachments,omitempty"`
}

func main() {
	flag.Parse()

	port := getDefaultConfig("PORT", "8080")
	username := getDefaultConfig("MG_USERNAME", "")
	password := getDefaultConfig("MG_PASSWORD", "")

	if username == "" || password == "" {
		fmt.Printf("No username or password supplied in the environment variables")
		os.Exit(1)
	}

	client := &http.Client{Timeout: 10 * time.Second}

	serveMux := http.NewServeMux()

	serveMux.HandleFunc("/", mainHandlerFor(username, password, client))

	srv := &http.Server{
		Addr:         ":" + port,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		// New for Go 1.8
		// IdleTimeout:  120 * time.Second,
		Handler: serveMux,
	}

	errorChan := make(chan error, 1)
	signalChan := make(chan os.Signal, 1)

	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		errorChan <- srv.ListenAndServe()
	}()

	for {
		select {
		case err := <-errorChan:
			if err != nil {
				log.Fatal(err)
			}
		case s := <-signalChan:
			log.Println(fmt.Sprintf("Captured %v. Exiting ...", s))
			// TOOD(jabley): shut down HTTP server cleanly
			os.Exit(0)
		}
	}
}

func getDefaultConfig(name, fallback string) string {
	if val := os.Getenv(name); val != "" {
		return val
	}
	return fallback
}

// Handles `POST /` and `POST /imageID[/]`
func mainHandlerFor(username, password string, client *http.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			w.WriteHeader(400)
			return
		}

		imageID := parseImageID(r)

		text := r.Form.Get("text")

		if text == "" {
			w.WriteHeader(400)
			return
		}

		args := strings.Split(text, ":")

		text0 := args[0]
		text1 := ""

		if len(args) > 1 {
			text1 = args[1]
		}

		imageURL, err := getImageURL(client, username, password, imageID, text0, text1)

		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
			return
		}

		payload := NewPayload(imageURL)

		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.Header().Set("Cache-Control", "private, no-cache, no-store, must-revalidate")

		json.NewEncoder(w).Encode(payload)
	}
}

func parseImageID(r *http.Request) string {
	imageID := "16191858"

	parts := Filter(strings.Split(r.URL.Path, "/"), func(s string) bool {
		return s != ""
	})

	if len(parts) > 0 {
		imageID = parts[0]
	}

	return imageID
}

func Filter(parts []string, fn func(string) bool) []string {
	res := make([]string, 0)
	for _, s := range parts {
		if fn(s) {
			res = append(res, s)
		}
	}
	return res
}

func getImageURL(client *http.Client, username, password, imageID, text0, text1 string) (*string, error) {
	u, err := url.Parse("http://version1.api.memegenerator.net/Instance_Create")
	if err != nil {
		return nil, err
	}

	v := url.Values{}
	v.Add("username", username)
	v.Add("password", password)
	v.Add("languageCode", "en")
	v.Add("text0", text0)
	v.Add("text1", text1)
	v.Add("imageID", imageID)
	v.Add("generatorID", "6693723")

	u.RawQuery = v.Encode()

	resp, err := client.Get(u.String())

	if err != nil {
		fmt.Printf("Failed to get successful response back\n")
		return nil, err
	}

	var doc interface{}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&doc)

	if err != nil {
		fmt.Printf("Failed to deserialise response body: %v\n", err)
		return nil, err
	}

	if m, ok := doc.(map[string]interface{}); ok == true {
		if res, ok := m["result"].(map[string]interface{}); ok == true {
			if URL, ok := res["instanceImageUrl"].(string); ok == true {
				return &URL, nil
			}
		}
	}

	return nil, errors.New(fmt.Sprintf("Unable to parse response JSON - %#v", doc))
}

func NewPayload(imageURL *string) *Payload {
	return &Payload{
		ResponseType: "in_channel",
		Attachments: []Attachment{
			{
				Text:     *imageURL,
				ImageURL: *imageURL,
			},
		},
	}
}
