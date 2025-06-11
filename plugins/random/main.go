package main

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"

	"github.com/rubiojr/sup/pkg/plugin"
)

type RandomPlugin struct{}

func (r *RandomPlugin) Name() string {
	return "random"
}

func (r *RandomPlugin) Topics() []string {
	return []string{"random"}
}

func (r *RandomPlugin) HandleMessage(input plugin.Input) plugin.Output {
	message := strings.TrimSpace(input.Message)

	if message == "" {
		return plugin.Error("Please provide at least one number. Usage: .sup random <max> or .sup random <min> <max>")
	}

	// Split the message to get the values
	parts := strings.Fields(message)

	var min, max int
	var err error

	// Handle one or two arguments
	if len(parts) == 1 {
		// Only one number provided, use 0 as min
		min = 0
		max, err = strconv.Atoi(parts[0])
		if err != nil {
			return plugin.Error(fmt.Sprintf("Invalid maximum value '%s'. Please provide a valid number.", parts[0]))
		}
	} else if len(parts) == 2 {
		// Two numbers provided
		min, err = strconv.Atoi(parts[0])
		if err != nil {
			return plugin.Error(fmt.Sprintf("Invalid minimum value '%s'. Please provide a valid number.", parts[0]))
		}

		max, err = strconv.Atoi(parts[1])
		if err != nil {
			return plugin.Error(fmt.Sprintf("Invalid maximum value '%s'. Please provide a valid number.", parts[1]))
		}
	} else {
		return plugin.Error("Too many arguments. Usage: .sup random <max> or .sup random <min> <max>")
	}

	// Ensure min is less than max
	if min >= max {
		return plugin.Error(fmt.Sprintf("Minimum value (%d) must be less than maximum value (%d).", min, max))
	}

	// Generate a random number
	randomNum := rand.IntN(max-min+1) + min

	return plugin.Success(fmt.Sprintf("🎲 %d", randomNum))
}

func (r *RandomPlugin) GetHelp() plugin.HelpOutput {
	return plugin.NewHelpOutput(
		"random",
		"Generate a random number between two values",
		".sup random <max> or .sup random <min> <max>",
		[]string{
			".sup random 10",
			".sup random 1 10",
			".sup random 100 999",
			".sup random -50 50",
		},
		"utility",
	)
}

func (r *RandomPlugin) GetRequiredEnvVars() []string {
	return []string{}
}

func (r *RandomPlugin) Version() string {
	return "0.1.0"
}

func init() {
	plugin.RegisterPlugin(&RandomPlugin{})
}

func main() {}
