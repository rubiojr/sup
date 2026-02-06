package handlers

import (
	"go.mau.fi/whatsmeow/types/events"
)

type Handler interface {
	HandleMessage(msg *events.Message) error
	GetHelp() HandlerHelp
	Name() string
	Topics() []string
	Version() string
}

// CLIHandler is an optional interface handlers can implement to expose CLI commands.
type CLIHandler interface {
	Handler
	HandleCLI(args []string) (string, error)
	SupportsCLI() bool
}

type HandlerHelp struct {
	Name        string
	Description string
	Usage       string
	Examples    []string
	Category    string
}
