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

	botInstance := bot.New(bot.WithLogger(logger), bot.WithTrigger(t))

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
	if err := b.RegisterHandler(handlers.NewMeteoHandler(b)); err != nil {
		return err
	}

	dd, err := botfs.HandlerDataDir("image-downloader")
	if err != nil {
		return err
	}
	imageDownloader := handlers.NewImageDownloaderHandler(dd)
	if err := b.RegisterHandler(imageDownloader); err != nil {
		return err
	}

	fd, err := botfs.HandlerDataDir("file-downloader")
	if err != nil {
		return err
	}
	fileDownloader := handlers.NewFileDownloaderHandler(fd)
	if err := b.RegisterHandler(fileDownloader); err != nil {
		return err
	}

	return nil
}
