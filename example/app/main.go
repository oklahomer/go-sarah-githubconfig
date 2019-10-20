package main

import (
	"context"
	"fmt"
	"github.com/oklahomer/go-sarah-githubconfig"
	_ "github.com/oklahomer/go-sarah-githubconfig/example/plugin/hello"
	"github.com/oklahomer/go-sarah/v2"
	"github.com/oklahomer/go-sarah/v2/slack"
	"golang.org/x/xerrors"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()

		// Give some interval to finish all chat integrations.
		time.Sleep(3 * time.Second)
	}()

	err := setupBot()
	if err != nil {
		panic(err)
	}

	err = setupWatcher(ctx)
	if err != nil {
		panic(err)
	}

	config := sarah.NewConfig()
	err = sarah.Run(ctx, config)
	if err != nil {
		panic(err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT)
	signal.Notify(c, syscall.SIGTERM)

	select {
	case <-c:
		fmt.Println("Finished")
		return

	}
}

func setupWatcher(ctx context.Context) error {
	cfg := githubconfig.NewConfig("oklahomer", "go-sarah-githubconfig-example", "config")
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return xerrors.New("GITHUB_TOKEN is not set")
	}
	watcher, err := githubconfig.New(ctx, cfg, githubconfig.WithToken(ctx, token))
	if err != nil {
		return xerrors.Errorf("failed to construct ConfigWatcher: %w", err)
	}

	sarah.RegisterConfigWatcher(watcher)
	return nil
}

func setupBot() error {
	config := slack.NewConfig()
	token := os.Getenv("SLACK_TOKEN")
	if token == "" {
		return xerrors.New("SLACK_TOKEN is not set")
	}
	config.Token = token

	adapter, err := slack.NewAdapter(config)
	if err != nil {
		return xerrors.Errorf("failed to construct an Adapter: %w", err)
	}
	bot, err := sarah.NewBot(adapter)
	if err != nil {
		return xerrors.Errorf("failed to construct a Bot: %w", err)
	}

	sarah.RegisterBot(bot)
	return nil
}
