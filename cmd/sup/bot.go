package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/urfave/cli/v3"

	"github.com/rubiojr/sup/bot"
	bothandlers "github.com/rubiojr/sup/bot/handlers"
	"github.com/rubiojr/sup/cmd/sup/handlers"
	"github.com/rubiojr/sup/internal/botfs"
	"github.com/rubiojr/sup/internal/config"
	"github.com/rubiojr/sup/internal/log"
)

var configFlag = &cli.StringFlag{
	Name:    "config",
	Aliases: []string{"c"},
	Usage:   "Path to bot config file",
	Value:   config.DefaultPath(),
}

var botCmd = &cli.Command{
	Name:  "bot",
	Usage: "Bot management commands",
	Commands: []*cli.Command{
		botRunCmd,
		botAllowListCmd,
	},
}

var botRunCmd = &cli.Command{
	Name:  "run",
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
		configFlag,
	},
	Action: botRunCommand,
}

var botAllowListCmd = &cli.Command{
	Name:    "allow-list",
	Aliases: []string{"al"},
	Usage:   "Manage allowed groups and users",
	Commands: []*cli.Command{
		{
			Name:    "edit",
			Aliases: []string{"e"},
			Usage:   "Edit allowed groups and users via TUI",
			Flags:   []cli.Flag{configFlag},
			Action:  botAllowListCommand,
		},
		{
			Name:    "list",
			Aliases: []string{"ls"},
			Usage:   "List currently allowed groups and users",
			Flags:   []cli.Flag{configFlag},
			Action:  botAllowListListCommand,
		},
	},
}

func botRunCommand(ctx context.Context, cmd *cli.Command) error {
	configPath := cmd.String("config")
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	t := cmd.String("trigger")
	// Config trigger is used unless overridden by CLI flag
	if !cmd.IsSet("trigger") && cfg.Trigger != "" {
		t = cfg.Trigger
	}

	debugLevel := cmd.Bool("debug")

	// Configure logger based on debug flag or config log_level
	if debugLevel {
		log.SetLevel(slog.LevelDebug)
	} else {
		switch cfg.LogLevel {
		case "debug":
			log.SetLevel(slog.LevelDebug)
		case "warn":
			log.SetLevel(slog.LevelWarn)
		case "error":
			log.SetLevel(slog.LevelError)
		}
	}
	logger := log.Default()

	botInstance, err := bot.New(
		bot.WithLogger(logger),
		bot.WithTrigger(t),
		bot.WithAllowedGroups(cfg.Allow.GroupJIDs()),
		bot.WithAllowedUsers(cfg.Allow.UserJIDs()),
		bot.WithAllowedCommands(cfg.Plugins.AllowedCommands),
	)
	if err != nil {
		return err
	}

	// Register all handlers
	if err := botInstance.RegisterDefaultHandlers(); err != nil {
		return err
	}

	if err := registerHandlers(botInstance, cfg); err != nil {
		return err
	}

	return botInstance.Start(ctx)
}

func registerHandlers(b *bot.Bot, cfg *config.Config) error {
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

	whatsAppLocationHandler := bothandlers.NewWhatsAppLocationHandler()
	if err := b.RegisterHandler(whatsAppLocationHandler); err != nil {
		return err
	}

	return nil
}

func botAllowListListCommand(_ context.Context, cmd *cli.Command) error {
	configPath := cmd.String("config")
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	if len(cfg.Allow.Groups) == 0 && len(cfg.Allow.Users) == 0 {
		fmt.Println("No allowed groups or users configured.")
		fmt.Printf("Config: %s\n", configPath)
		return nil
	}

	if len(cfg.Allow.Groups) > 0 {
		fmt.Println("Allowed Groups:")
		for _, g := range cfg.Allow.Groups {
			if g.Name != "" {
				fmt.Printf("  %s (%s)\n", g.Name, g.JID)
			} else {
				fmt.Printf("  %s\n", g.JID)
			}
		}
	}

	if len(cfg.Allow.Users) > 0 {
		fmt.Println("Allowed Users:")
		for _, u := range cfg.Allow.Users {
			if u.Name != "" {
				fmt.Printf("  %s (%s)\n", u.Name, u.JID)
			} else {
				fmt.Printf("  %s\n", u.JID)
			}
		}
	}

	return nil
}
