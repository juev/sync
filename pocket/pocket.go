package pocket

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type Pocket struct {
	ConsumerKey string `json:"consumer_key"`
	AccessToken string `json:"access_token"`
	State       string `json:"state"`
	DetailType  string `json:"detailType"`
	Count       int    `json:"count"`
	Offset      int    `json:"offset"`
	Total       int    `json:"total"`
	body        string
	client      *http.Client
}

const (
	endpoint            = "https://getpocket.com/v3/get"
	pocketCount         = 30
	pocketTotal         = 1
	pocketDefaultOffset = 0
	pocketState         = "unread"
	pocketDetailType    = "simple"
)

var (
	ErrEmptyList          = errors.New("empty list")
	ErrSomethingWentWrong = errors.New("Something Went Wrong")
)

func New(consumerKey, accessToken string) (*Pocket, error) {
	body, _ := json.Marshal(&Pocket{
		ConsumerKey: consumerKey,
		AccessToken: accessToken,
		State:       pocketState,
		DetailType:  pocketDetailType,
		Count:       pocketCount,
		Offset:      pocketDefaultOffset,
		Total:       pocketTotal,
	})

	client := &http.Client{
		Timeout: time.Second * 10,
	}

	return &Pocket{
		body:   string(body),
		client: client,
	}, nil
}

func (p *Pocket) Retrive(since int64) ([]string, error) {
	request, _ := http.NewRequest(http.MethodPost, endpoint, nil)
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("X-Accept", "application/json")

	operation := func(offset int) ([]string, error) {
		body := p.body
		body, _ = sjson.Set(body, "since", since)
		body, _ = sjson.Set(body, "offset", offset)
		request.Body = io.NopCloser(strings.NewReader(body))
		response, err := p.client.Do(request)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()

		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("got response %d; X-Error=[%s]", response.StatusCode, response.Header.Get("X-Error"))
		}

		bodyBytes, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}
		bodyString := string(bodyBytes)
		if e := gjson.Get(bodyString, "error").String(); e != "" {
			return nil, ErrSomethingWentWrong
		}

		if gjson.Get(bodyString, "status").Int() == 2 {
			return nil, ErrEmptyList
		}

		list := gjson.Get(bodyString, "list").Map()
		var result []string
		for k := range list {
			value := list[k].String()
			u := gjson.Get(value, "resolved_url")
			if u.String() == "" {
				u = gjson.Get(value, "given_url")
			}
			if u.Exists() {
				result = append(result, u.String())
			}
		}

		return result, nil
	}

	retrive := func(offset int) ([]string, error) {
		var (
			err   error
			links []string
		)

		ticker := backoff.NewTicker(backoff.NewExponentialBackOff())
		defer ticker.Stop()
		for range ticker.C {
			links, err = operation(offset)
			if errors.Is(err, ErrSomethingWentWrong) {
				break
			}
			if err != nil && !errors.Is(err, ErrEmptyList) {
				continue
			}

			break
		}

		if err != nil {
			return nil, err
		}

		return links, nil
	}

	offset := pocketDefaultOffset
	var (
		result []string
		err    error
	)

	count := pocketCount
	for count > 0 {
		var links []string
		links, err = retrive(offset)
		if err != nil {
			return nil, err
		}
		count = len(links)
		if count > 0 {
			result = append(result, links...)
		}
		offset += pocketCount
	}

	return result, nil
}
