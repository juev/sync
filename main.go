package main

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/carlmjohnson/requests"
	"github.com/cenkalti/backoff/v4"
	"github.com/juev/sync/prettylog"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	DEFAULT_SCHEDULE_TIME = "30m"
)

var logger *slog.Logger

var level = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

var linkdingUnauthorizedErr = errors.New("Linkding Unauthorized")

func main() {
	// Iintialize logger
	logLevelEnv := os.Getenv("LOG_LEVEL")
	logLevel, ok := level[strings.ToLower(logLevelEnv)]
	if !ok {
		logLevel = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{
		Level: logLevel,
	}
	logger = slog.New(prettylog.NewHandler(opts))

	// Start
	logger.Info("Starting")

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
	linkdingUrl := os.Getenv("LINKDING_URL")
	if linkdingUrl == "" {
		logger.Error("LINKDING_URL is not set")
		os.Exit(1)
	}

	sheduleTimeEnv := cmp.Or(os.Getenv("SCHEDULE_TIME"), DEFAULT_SCHEDULE_TIME)
	sheduleTime, err := time.ParseDuration(sheduleTimeEnv)
	if err != nil {
		sheduleTime, _ = time.ParseDuration(DEFAULT_SCHEDULE_TIME)
	}

	// First run operation
	err = process(
		pocketConsumerKey,
		pocketAccessToken,
		linkdingAccessToken,
		linkdingUrl,
		sheduleTime,
	)
	if err != nil {
		logger.Error("Failed process", "error", err)
		os.Exit(1)
	}

	// Create a ticker that triggers every 30 minutes
	ticker := time.NewTicker(sheduleTime)
	defer ticker.Stop()

	// Create a channel to listen for system signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			err = process(
				pocketConsumerKey,
				pocketAccessToken,
				linkdingAccessToken,
				linkdingUrl,
				sheduleTime,
			)
			if err != nil {
				logger.Error("Failed process", "error", err)
				os.Exit(1)
			}
		case <-sigChan:
			logger.Info("Received shutdown signal")
			return
		}
	}
}

func process(pocketConsumerKey, pocketAccessToken, linkdingAccessToken, linkdingUrl string,
	sheduleTime time.Duration) error {
	since := time.Now().Add(sheduleTime).Unix()

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
			logger.Error("Failed to fetch getpocket data", "error", err)
			return "", err
		}

		return dat, nil
	}

	dat, err := backoff.RetryWithData(operation, backoff.NewExponentialBackOff())
	if err != nil {
		return err
	}

	if gjson.Get(dat, "status").Int() == 2 {
		logger.Info("No new items")
		return nil
	}

	list := gjson.Get(dat, "list").Map()
	for k := range list {
		value := list[k].String()
		u := gjson.Get(value, "resolved_url")
		if u.Exists() {
			logger.Info("Processing", "resolved_url", u.String())

			operation := func() error {
				value, _ := sjson.Set("", "url", u.String())
				err := requests.
					URL(linkdingUrl+"/api/bookmarks/").
					BodyBytes([]byte(value)).
					Header("Authorization", "Token "+linkdingAccessToken).
					ContentType("application/json").
					Fetch(context.Background())
				if requests.HasStatusErr(err, http.StatusUnauthorized) {
					return backoff.Permanent(linkdingUnauthorizedErr)
				}

				if err != nil {
					return err
				}

				return nil
			}

			err := backoff.Retry(operation, backoff.NewExponentialBackOff())
			if err != nil {
				logger.Error("Failed to save bookmark", "error", err, "resolved_url", u.String())
				return err
			}

			logger.Info("Added", "url", u.String())
		}
	}

	return nil
}
