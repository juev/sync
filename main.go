package main

import (
	"cmp"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/juev/sync/linkding"
	"github.com/juev/sync/pocket"
	"github.com/juev/sync/prettylog"
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

	pocketClient, err := pocket.New(pocketConsumerKey, pocketAccessToken)
	if err != nil {
		logger.Error("Failed to create Pocket client", "error", err)
		os.Exit(1)
	}
	linkdingClient, err := linkding.New(linkdingURL, linkdingAccessToken)
	if err != nil {
		logger.Error("Failed to create Linkding client", "error", err)
		os.Exit(1)
	}

	// Start
	logger.Info("Starting")

	runProcess := func(since int64) int64 {
		logger.Debug("Processing", "since", time.Unix(since, 0).Format(time.RFC3339))
		newSince := time.Now().Unix()
		links, err := pocketClient.Retrive(since)
		if err == pocket.ErrEmptyList {
			logger.Info("No new links")
			return newSince
		}
		if err != nil {
			logger.Error("Failed to retrieve Pocket data", "error", err)
			return since
		}
		for _, link := range links {
			if err := linkdingClient.Add(link); err != nil {
				logger.Error("Failed to save bookmark", "error", err)
				return since
			}
		}
		logger.Info("Processed", "count", len(links))
		return newSince
	}

	// 30 days ago
	since := time.Now().Add(-24 * 30 * time.Hour).Unix()
	since = runProcess(since)

	// Create a ticker that triggers every sheduleTime value
	ticker := time.NewTicker(scheduleTime)
	defer ticker.Stop()

	for range ticker.C {
		since = runProcess(since)
	}
}
