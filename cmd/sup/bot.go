package main

import (
	"context"
	"log/slog"

	"github.com/urfave/cli/v3"

	"github.com/rubiojr/sup/bot"
	"github.com/rubiojr/sup/cmd/sup/handlers"
	"github.com/rubiojr/sup/internal/botfs"
	"github.com/rubiojr/sup/internal/log"
)

var botCmd = &cli.Command{
	Name:  "bot",
	Usage: "Start bot mode to listen for messages and run command handlers",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "trigger",
			Aliases: []string{"t"},
			Usage:   "Command prefix to trigger bot handlers",
			Value:   ".sup",
		},
		&cli.BoolFlag{
			Name:    "debug",
			Aliases: []string{"d"},
			Usage:   "Debug level",
		},
	},
	Action: botCommand,
}

func botCommand(ctx context.Context, cmd *cli.Command) error {
	t := cmd.String("trigger")
	debugLevel := cmd.Bool("debug")

	// Configure logger based on debug level
	if debugLevel {
		log.SetLevel(slog.LevelDebug)
	}
	logger := log.Default()

	botInstance, err := bot.New(bot.WithLogger(logger), bot.WithTrigger(t))
	if err != nil {
		return err
	}

	// Register all handlers
	if err := botInstance.RegisterDefaultHandlers(); err != nil {
		return err
	}

	if err := registerHandlers(botInstance); err != nil {
		return err
	}

	return botInstance.Start(ctx)
}

func registerHandlers(b *bot.Bot) error {
	cache, err := b.Cache()
	if err != nil {
		return err
	}
	if err := b.RegisterHandler(handlers.NewMeteoHandler(cache.Namespace("meteo"))); err != nil {
		return err
	}

	store, err := b.Store()
	if err != nil {
		return err
	}
	if err := b.RegisterHandler(handlers.NewRemindersHandler(store.Namespace("reminders"))); err != nil {
		return err
	}

	dd := botfs.HandlerDataDir("image-downloader")
	imageDownloader := handlers.NewImageDownloaderHandler(dd)
	if err := b.RegisterHandler(imageDownloader); err != nil {
		return err
	}

	fd := botfs.HandlerDataDir("file-downloader")
	fileDownloader := handlers.NewFileDownloaderHandler(fd)
	if err := b.RegisterHandler(fileDownloader); err != nil {
		return err
	}

	return nil
}
