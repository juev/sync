package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/carlmjohnson/requests"
	"github.com/cenkalti/backoff/v5"
	"github.com/tidwall/gjson"
)

func main() {
	consumer_key := os.Getenv("POCKET_CONSUMER_KEY")
	if consumer_key == "" {
		fmt.Println("POCKET_CONSUMER_KEY is not set")
		os.Exit(1)
	}
	access_token := os.Getenv("POCKET_ACCESS_TOKEN")
	if access_token == "" {
		fmt.Println("POCKET_ACCESS_TOKEN is not set")
		os.Exit(1)
	}

	since := time.Now().Add(30 * time.Minute).Unix()

	operation := func() (string, error) {
		var dat string
		err := requests.
			URL("https://getpocket.com/v3/get").
			Param("consumer_key", consumer_key).
			Param("access_token", access_token).
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
		fmt.Println("Error:", err)
		return
	}

	fmt.Println(result)
	gjson.Get(result, "list").ForEach(func(key, value gjson.Result) bool {
		u := value.Get("resolved_url")
		if u.Exists() {
			fmt.Println(u.String())
		}

		return true
	})

}
