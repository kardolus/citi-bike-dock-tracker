package http

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

const (
	contentType              = "application/json"
	errFailedToRead          = "failed to read response: %w"
	errFailedToCreateRequest = "failed to create request: %w"
	errFailedToMakeRequest   = "failed to make request: %w"
	errHTTP                  = "http error: %d"
	headerContentType        = "Content-Type"
)

type Caller interface {
	Get(url string) ([]byte, error)
}

type RestCaller struct {
	client *http.Client
}

// Ensure RestCaller implements Caller interface
var _ Caller = &RestCaller{}

func New() *RestCaller {
	return &RestCaller{
		client: &http.Client{},
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

	return req, nil
}
