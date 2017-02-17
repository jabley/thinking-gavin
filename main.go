package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type attachment struct {
	Text     string `json:"text"`
	ImageURL string `json:"image_url"`
}

type payload struct {
	ResponseType string       `json:"response_type"`
	Attachments  []attachment `json:"attachments,omitempty"`
}

type errorInfo struct {
	ID     string   `json:"id,omitempty"`
	HREF   string   `json:"href,omitempty"`
	Status string   `json:"status,omitempty"`
	Code   string   `json:"code,omitempty"`
	Title  string   `json:"title,omitempty"`
	Detail string   `json:"detail,omitempty"`
	Links  []string `json:"links,omitempty"`
	Path   string   `json:"path,omitempty"`
}

type errorResponse struct {
	Errors []errorInfo `json:"errors,omitempty"`
}

// ErrBadRequest is used to report when the client hasn't called the service
// correctly. Think of it as 'usage' for a web service.
var ErrBadRequest = errors.New("Bad request - no text provided")

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
		IdleTimeout:  120 * time.Second,
		Handler:      serveMux,
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
			d := time.Now().Add(1 * time.Second)
			ctx, cancel := context.WithDeadline(context.Background(), d)
			defer cancel()
			srv.Shutdown(ctx)
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
		ctxt, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := r.ParseForm()
		if err != nil {
			renderError(w, 400, ErrBadRequest)
			return
		}

		imageID := parseImageID(r)

		text := r.Form.Get("text")

		if text == "" {
			renderError(w, 400, ErrBadRequest)
			return
		}

		args := strings.Split(text, ":")

		text0 := args[0]
		text1 := ""

		if len(args) > 1 {
			text1 = args[1]
		}

		imageURL, err := getImageURL(ctxt, client, username, password, imageID, text0, text1)

		if err != nil {
			renderError(w, 500, err)
			return
		}

		renderSuccess(w, newPayload(imageURL))
	}
}

func renderSuccess(w http.ResponseWriter, payload *payload) {
	writeStatus(w, 200)
	writeJSON(w, payload)
}

func renderError(w http.ResponseWriter, status int, err error) {
	writeStatus(w, status)
	writeJSON(w, &errorResponse{
		Errors: []errorInfo{
			{
				Detail: err.Error(),
				Status: strconv.Itoa(status),
			},
		},
	})
}

func writeJSON(w http.ResponseWriter, JSON interface{}) {
	json.NewEncoder(w).Encode(JSON)
}

func writeStatus(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Header().Set("Cache-Control", "private, no-cache, no-store, must-revalidate")
	w.WriteHeader(status)
}

func parseImageID(r *http.Request) string {
	imageID := "16191858"

	parts := filter(strings.Split(r.URL.Path, "/"), func(s string) bool {
		return s != ""
	})

	if len(parts) > 0 {
		imageID = parts[0]
	}

	return imageID
}

func filter(parts []string, fn func(string) bool) []string {
	var res []string
	for _, s := range parts {
		if fn(s) {
			res = append(res, s)
		}
	}
	return res
}

func getImageURL(ctxt context.Context, client *http.Client, username, password, imageID, text0, text1 string) (*string, error) {
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

	return nil, fmt.Errorf("Unable to parse response JSON - %#v", doc)
}

func newPayload(imageURL *string) *payload {
	return &payload{
		ResponseType: "in_channel",
		Attachments: []attachment{
			{
				Text:     *imageURL,
				ImageURL: *imageURL,
			},
		},
	}
}
