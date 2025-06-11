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

type HandlerHelp struct {
	Name        string
	Description string
	Usage       string
	Examples    []string
	Category    string
}
