package main

import (
	"cmp"
	"context"
	"log/slog"
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

	// Init backoff
	expBackOff := backoff.NewExponentialBackOff()
	expBackOff.MaxElapsedTime = 5 * time.Minute
	operation := func() error {
		return process(
			pocketConsumerKey,
			pocketAccessToken,
			linkdingAccessToken,
			linkdingUrl,
			sheduleTime,
		)
	}

	// First run operation
	if err := backoff.Retry(operation, expBackOff); err != nil {
		logger.Error("Failed process", "error", err)
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
			expBackOff.Reset()
			err := backoff.Retry(operation, expBackOff)
			if err != nil {
				logger.Error("Failed process", "error", err)
			}
		case <-sigChan:
			logger.Info("Received shutdown signal")
			return
		}
	}
}

func process(pocketConsumerKey, pocketAccessToken, linkdingAccessToken, linkdingUrl string, sheduleTime time.Duration) error {
	since := time.Now().Add(sheduleTime).Unix()

	var dat string
	err := requests.
		URL("https://getpocket.com/v3/get").
		Param("consumer_key", pocketConsumerKey).
		Param("access_token", pocketAccessToken).
		Param("since", strconv.FormatInt(since, 10)).
		ToString(&dat).
		Fetch(context.Background())
	if err != nil {
		logger.Error("Error", "error", err)
		return err
	}

	if gjson.Get(dat, "status").Int() == 2 {
		logger.Info("No new items")
		return nil
	}

	gjson.Get(dat, "list").ForEach(func(_, value gjson.Result) bool {
		u := value.Get("resolved_url")
		if u.Exists() {
			value, _ := sjson.Set("", "url", u.String())
			err := requests.
				URL(linkdingUrl+"/api/bookmarks/").
				BodyBytes([]byte(value)).
				Header("auth", "Authorization "+linkdingAccessToken).
				ContentType("application/json").
				Fetch(context.Background())
			if err != nil {
				logger.Error("Error", "error", err)
				return false
			}

			logger.Info("Added", "url", u.String())
		}

		return true
	})

	return nil
}
