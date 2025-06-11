package main

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/urfave/cli/v3"
)

var (
	Version = "0.1.2"
)

var versionCmd = &cli.Command{
	Name:    "version",
	Aliases: []string{"v"},
	Usage:   "Show version information",
	Action:  versionCommand,
}

func versionCommand(ctx context.Context, cmd *cli.Command) error {
	buildDate := getBuildDate()
	fmt.Printf("sup version %s\n", Version)
	fmt.Printf("Build date: %s\n", buildDate)
	fmt.Printf("Go version: %s\n", runtime.Version())
	fmt.Printf("OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	return nil
}

func getBuildDate() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}

	for _, setting := range info.Settings {
		if setting.Key == "vcs.time" {
			return setting.Value
		}
	}

	return "unknown"
}
