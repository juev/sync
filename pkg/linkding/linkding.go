package linkding

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/juev/sync/internal/client"
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
	}, nil
}

func (l *Linkding) Add(u string) error {
	body, _ := sjson.Set("", "url", u)
	request := l.request
	request.Body = io.NopCloser(strings.NewReader(body))

	response, err := client.Request(&request)
	if err != nil {
		return fmt.Errorf("failed to send request to linkding: %w", err)
	}

	if response.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("got response %d; X-Error=[%s]", response.StatusCode, response.Header.Get("X-Error"))
	}

	return nil
}
