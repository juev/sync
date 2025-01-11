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
)

const (
	defaultScheduleTime = "30m"
)

var logger *slog.Logger

var logLevels = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

var errLinkdingUnauthorized = errors.New("Linkding Unauthorized")

var since int64

func main() {
	// Initialize logger
	logLevelEnv := os.Getenv("LOG_LEVEL")
	logLevel, ok := logLevels[strings.ToLower(logLevelEnv)]
	if !ok {
		logLevel = slog.LevelInfo // Default log level
	}

	// Initialize logger with the determined log level
	logger = slog.New(prettylog.NewHandler(&slog.HandlerOptions{Level: logLevel}))

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		logger.Info("Received signal, shutting down", "signal", sig)
		os.Exit(0)
	}()

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
	linkdingURL := os.Getenv("LINKDING_URL")
	if linkdingURL == "" {
		logger.Error("LINKDING_URL is not set")
		os.Exit(1)
	}

	scheduleTimeEnv := cmp.Or(os.Getenv("SCHEDULE_TIME"), defaultScheduleTime)
	scheduleTime, err := time.ParseDuration(scheduleTimeEnv)
	if err != nil {
		scheduleTime, _ = time.ParseDuration(defaultScheduleTime)
	}

	// Start
	logger.Info("Starting")

	// First run operation
	runProcess := func() {
		err = process(
			pocketConsumerKey,
			pocketAccessToken,
			linkdingAccessToken,
			linkdingURL,
		)
		if err != nil {
			logger.Error("Failed process", "error", err)
		}
	}
	// 30 days ago
	since = time.Now().Add(-24 * 30 * time.Hour).Unix()
	runProcess()

	// Create a ticker that triggers every sheduleTime value
	ticker := time.NewTicker(scheduleTime)
	defer ticker.Stop()

	for range ticker.C {
		runProcess()
	}
}

func process(pocketConsumerKey, pocketAccessToken, linkdingAccessToken, linkdingURL string) error {
	logger.Debug("Requesting Pocket data", "since", time.Unix(since, 0).Format(time.RFC3339))
	operation := func() (string, error) {
		var responseData string
		err := requests.
			URL("https://getpocket.com/v3/get").
			BodyJSON(&requestPocket{
				State:       "unread",
				DetailType:  "simple",
				ConsumerKey: pocketConsumerKey,
				AccessToken: pocketAccessToken,
				Since:       strconv.FormatInt(since, 10),
			}).
			ContentType("application/json").
			ToString(&responseData).
			Fetch(context.Background())
		if err != nil {
			logger.Error("Failed to fetch getpocket data", "error", err)
			return "", err
		}

		return responseData, nil
	}

	newSince := time.Now().Unix()
	responseData, err := backoff.RetryWithData(operation, backoff.NewExponentialBackOff())
	if err != nil {
		logger.Error("Failed request to Pocket", "error", err)
		return err
	}

	if e := gjson.Get(responseData, "error").String(); e != "" {
		return errors.New(e)
	}

	if gjson.Get(responseData, "status").Int() == 2 {
		logger.Info("No new data from Pocket")
		since = newSince
		return nil
	}

	list := gjson.Get(responseData, "list").Map()
	var exitErr error
	var count int
	for k := range list {
		value := list[k].String()
		u := gjson.Get(value, "resolved_url")
		if u.Exists() {
			logger.Info("Processing", "resolved_url", u.String())

			operation := func() error {
				err := requests.
					URL(linkdingURL+"/api/bookmarks/").
					BodyJSON(&requestLinkding{
						URL: u.String(),
					}).
					Header("Authorization", "Token "+linkdingAccessToken).
					ContentType("application/json").
					Fetch(context.Background())
				if requests.HasStatusErr(err, http.StatusUnauthorized) {
					return backoff.Permanent(errLinkdingUnauthorized)
				}

				if err != nil {
					return err
				}

				return nil
			}

			err := backoff.Retry(operation, backoff.NewExponentialBackOff())
			if errors.Is(err, errLinkdingUnauthorized) {
				return err
			}
			if err != nil {
				logger.Error("Failed to save bookmark", "error", err, "resolved_url", u.String())
				if !errors.Is(exitErr, err) {
					exitErr = errors.Join(exitErr, err)
				}

				continue
			}

			count++
			logger.Info("Added", "url", u.String())
		}
	}
	logger.Info("Processed", "count", count)

	if exitErr == nil {
		since = newSince
	}

	return exitErr
}

type requestPocket struct {
	ConsumerKey string `json:"consumer_key"`
	AccessToken string `json:"access_token"`
	State       string `json:"state"`
	DetailType  string `json:"detailType"`
	Since       string `json:"since"`
}

type requestLinkding struct {
	URL string `json:"url"`
}
