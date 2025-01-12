package linkding

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/tidwall/sjson"
)

type Linkding struct {
	request http.Request
	client  *http.Client
}

var (
	ErrLinkdingUnauthorized = errors.New("Linkding Unauthorized")
)

func New(apiURL, token string) (*Linkding, error) {
	client := &http.Client{
		Timeout: time.Second * 10,
	}

	u, err := url.Parse(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed parse linkding apiUrl: %w", err)
	}
	u = u.ResolveReference(&url.URL{Path: "/api/bookmarks/"})

	request, _ := http.NewRequest(http.MethodPost, u.String(), nil)
	request.Header.Add("Authorization", "Token "+token)
	request.Header.Add("Content-Type", "application/json")

	return &Linkding{
		request: *request,
		client:  client,
	}, nil
}

func (l *Linkding) Add(u string) error {
	body, _ := sjson.Set("", "url", u)
	request := l.request
	request.Body = io.NopCloser(strings.NewReader(body))

	operation := func() error {
		response, err := l.client.Do(&request)
		if err != nil {
			return fmt.Errorf("failed to send request to linkding: %w", err)
		}
		defer response.Body.Close()

		if response.StatusCode == http.StatusUnauthorized {
			return backoff.Permanent(ErrLinkdingUnauthorized)
		}

		return nil
	}

	return backoff.Retry(operation, backoff.NewExponentialBackOff())
}
