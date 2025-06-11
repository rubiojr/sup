package main

import (
	"context"
	"log"
	"os"

	"github.com/urfave/cli/v3"
)

func main() {
	app := &cli.Command{
		Name:  "sup",
		Usage: "WhatsApp file sender CLI",
		Commands: []*cli.Command{
			registerCmd,
			statusCmd,
			listGroupCmd,
			listContactCmd,
			sendFileCmd,
			sendCmd,
			sendImageCmd,
			sendClipboardCmd,
			botCmd,
			pluginsCmd,
			registryCmd,
			versionCmd,
		},
	}
	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
