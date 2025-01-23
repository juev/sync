package pocket

import (
	"github.com/juev/getpocket"
)

type Pocket struct {
	client *getpocket.Pocket
}

func New(consumerKey, accessToken string) (*Pocket, error) {
	client, err := getpocket.New(consumerKey, accessToken)
	if err != nil {
		return nil, err
	}

	return &Pocket{
		client: client,
	}, nil
}

func (p *Pocket) Retrive(since int64) ([]string, int64, error) {
	items, newSince, err := p.client.Retrive(since)
	if err != nil {
		return nil, newSince, err
	}

	var result []string
	for _, item := range items {
		result = append(result, item.ResolvedURL)
	}

	return result, newSince, nil
}
