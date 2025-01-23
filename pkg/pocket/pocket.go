package pocket

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/juev/sync/internal/client"
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

func (p *Pocket) Retrive(since int64) ([]string, int64, error) {
	offset := pocketDefaultOffset
	var (
		newSince int64
		result   []string
		err      error
	)

	count := pocketCount
	for count > 0 {
		var links []string
		links, newSince, err = p.operation(since, offset)
		if err != nil {
			return nil, 0, err
		}
		count = len(links)
		if count > 0 {
			result = append(result, links...)
		}
		offset += pocketCount
	}

	return result, newSince, nil
}

func (p *Pocket) operation(since int64, offset int) ([]string, int64, error) {
	request, _ := http.NewRequest(http.MethodPost, endpoint, nil)
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("X-Accept", "application/json")

	body := p.body
	body, _ = sjson.Set(body, "since", since)
	body, _ = sjson.Set(body, "offset", offset)
	request.Body = io.NopCloser(strings.NewReader(body))
	response, err := client.Request(request)
	if err != nil {
		return nil, 0, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("got response %d; X-Error=[%s]", response.StatusCode, response.Header.Get("X-Error"))
	}

	bodyString := response.Body

	if gjson.Get(bodyString, "error").String() != "" {
		return nil, 0, fmt.Errorf("got response %d; X-Error=[%s]", response.StatusCode, response.Header.Get("X-Error"))
	}

	// Update since
	newSince := gjson.Get(bodyString, "since").Int()

	if gjson.Get(bodyString, "status").Int() == 2 {
		return nil, newSince, nil
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

	return result, newSince, nil
}
