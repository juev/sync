package main

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/carlmjohnson/requests"
	"github.com/cenkalti/backoff/v5"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var logger *slog.Logger

func init() {
	logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
}

func main() {
	pocketConsumerKey := os.Getenv("POCKET_CONSUMER_KEY")
	if pocketConsumerKey == "" {
		logger.Error("POCKET_CONSUMER_KEY is not set")
		os.Exit(1)
	}
	pocketAccessToken := os.Getenv("POCKET_ACCESS_TOKEN")
	if pocketAccessToken == "" {
		logger.Error("POCKET_ACCESS_TOKEN is not set")
		os.Exit(1)
	}
	linkdingAccessToken := os.Getenv("LINKDING_ACCESS_TOKEN")
	if linkdingAccessToken == "" {
		logger.Error("LINKDING_ACCESS_TOKEN is not set")
		os.Exit(1)
	}

	if err := process(
		pocketConsumerKey,
		pocketAccessToken,
		linkdingAccessToken,
	); err != nil {
		logger.Error("Error", "error", err)
		os.Exit(1)
	}

}

func process(pocketConsumerKey, pocketAccessToken, linkdingAccessToken string) error {
	since := time.Now().Add(30 * time.Minute).Unix()

	operation := func() (string, error) {
		var dat string
		err := requests.
			URL("https://getpocket.com/v3/get").
			Param("consumer_key", pocketConsumerKey).
			Param("access_token", pocketAccessToken).
			Param("since", strconv.FormatInt(since, 10)).
			ToString(&dat).
			Fetch(context.Background())
		if err != nil {
			return "", backoff.RetryAfter(1)
		}

		return dat, nil
	}

	result, err := backoff.Retry(context.TODO(), operation, backoff.WithBackOff(backoff.NewExponentialBackOff()))
	if err != nil {
		logger.Error("Error", "error", err)
		return err
	}

	if gjson.Get(result, "status").Int() == 2 {
		logger.Info("No new items")
		return nil
	}

	gjson.Get(result, "list").ForEach(func(key, value gjson.Result) bool {
		u := value.Get("resolved_url")
		if u.Exists() {
			value, _ := sjson.Set("", "url", u.String())
			operation := func() (string, error) {
				err := requests.
					URL("https://links.evsyukov.org/api/bookmarks/").
					BodyBytes([]byte(value)).
					Header("auth", "Authorization "+linkdingAccessToken).
					ContentType("application/json").
					Fetch(context.Background())
				if err != nil {
					return "", backoff.RetryAfter(1)
				}

				return "", nil
			}

			_, err := backoff.Retry(context.TODO(), operation, backoff.WithBackOff(backoff.NewExponentialBackOff()))
			if err != nil {
				logger.Error("Error", "error", err)
				return false
			}
		}

		logger.Info("Added", "url", u.String())

		return true
	})

	return nil
}
