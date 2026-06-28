package http

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	contentType              = "application/json"
	errFailedToRead          = "failed to read response: %w"
	errFailedToCreateRequest = "failed to create request: %w"
	errFailedToMakeRequest   = "failed to make request: %w"
	errHTTP                  = "http error: %d"
	headerContentType        = "Content-Type"
	defaultUserAgent         = "dockscan-bikeshare-tracker/1.0 (+https://kardol.us; unofficial, public GBFS)"
)

// userAgent identifies the poller to feed operators (good citizenship at N cities);
// override with the USER_AGENT env var.
var userAgent = func() string {
	if v := os.Getenv("USER_AGENT"); v != "" {
		return v
	}
	return defaultUserAgent
}()

type Caller interface {
	Get(url string) ([]byte, error)
}

type RestCaller struct {
	client *http.Client
}

// Ensure RestCaller implements Caller interface
var _ Caller = &RestCaller{}

func New() *RestCaller {
	// bounded timeout so a stuck GBFS connection can't hang the ingest loop
	return &RestCaller{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (r *RestCaller) Get(url string) ([]byte, error) {
	return r.doRequest(http.MethodGet, url, nil)
}

func (r *RestCaller) doRequest(method, url string, body []byte) ([]byte, error) {
	req, err := r.newRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf(errFailedToCreateRequest, err)
	}

	response, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf(errFailedToMakeRequest, err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf(errHTTP, response.StatusCode)
	}

	result, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf(errFailedToRead, err)
	}

	return result, nil
}

func (r *RestCaller) newRequest(method, url string, body []byte) (*http.Request, error) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set(headerContentType, contentType)
	req.Header.Set("User-Agent", userAgent)

	return req, nil
}
