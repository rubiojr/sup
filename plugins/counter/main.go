package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rubiojr/sup/pkg/plugin"
)

type CounterPlugin struct{}

func (p *CounterPlugin) Name() string {
	return "counter"
}

func (p *CounterPlugin) Topics() []string {
	return []string{"counter"}
}

func (p *CounterPlugin) HandleMessage(input plugin.Input) plugin.Output {
	args := strings.Fields(input.Message)
	var action string
	var counterName string

	if len(args) > 0 {
		action = args[0]
	}
	if len(args) > 1 {
		counterName = args[1]
	} else {
		counterName = "default"
	}

	storeKey := fmt.Sprintf("%s:%s", input.Sender, counterName)
	switch action {
	case "increment", "inc", "+":
		return p.incrementCounter(storeKey)
	case "decrement", "dec", "-":
		return p.decrementCounter(storeKey)
	case "reset":
		return p.resetCounter(storeKey)
	case "list":
		return p.listCounters(input.Sender)
	default:
		return p.getCounter(storeKey)
	}
}

func (p *CounterPlugin) incrementCounter(storeKey string) plugin.Output {
	count, err := p.getCurrentCount(storeKey)
	if err != nil {
		count = 0
	}

	count++

	err = p.storeCount(storeKey, count)
	if err != nil {
		return plugin.Error("Failed to store counter: " + err.Error())
	}

	return plugin.Success(fmt.Sprintf("Counter incremented to %d", count))
}

func (p *CounterPlugin) decrementCounter(storeKey string) plugin.Output {
	count, err := p.getCurrentCount(storeKey)
	if err != nil {
		count = 0
	}

	if count > 0 {
		count--
	}

	err = p.storeCount(storeKey, count)
	if err != nil {
		return plugin.Error("Failed to store counter: " + err.Error())
	}

	return plugin.Success(fmt.Sprintf("Counter decremented to %d", count))
}

func (p *CounterPlugin) resetCounter(storeKey string) plugin.Output {
	err := p.storeCount(storeKey, 0)
	if err != nil {
		return plugin.Error("Failed to reset counter: " + err.Error())
	}

	return plugin.Success("Counter reset to 0")
}

func (p *CounterPlugin) getCounter(storeKey string) plugin.Output {
	count, err := p.getCurrentCount(storeKey)
	if err != nil {
		return plugin.Success("Counter value: 0 (not set)")
	}

	return plugin.Success(fmt.Sprintf("Counter value: %d", count))
}

func (p *CounterPlugin) listCounters(sender string) plugin.Output {
	// Note: This is a simplified implementation. In a real scenario,
	// you might want to implement a way to list all keys with a prefix
	// For now, we'll just return a message about the limitation
	return plugin.Success("Counter listing not implemented yet. Use specific counter names or 'default'.")
}

func (p *CounterPlugin) getCurrentCount(storeKey string) (int, error) {
	data, err := plugin.Storage().Get(storeKey)
	if err != nil {
		return 0, err
	}
	if data == nil {
		return 0, fmt.Errorf("key not found in store: %s", storeKey)
	}

	count, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, fmt.Errorf("failed to parse counter value from store: %w", err)
	}

	return count, nil
}

func (p *CounterPlugin) storeCount(storeKey string, count int) error {
	countStr := strconv.Itoa(count)
	return plugin.Storage().Set(storeKey, []byte(countStr))
}

func (p *CounterPlugin) GetHelp() plugin.HelpOutput {
	return plugin.NewHelpOutput(
		"counter",
		"Manages counters for users",
		".sup counter [action] [name]",
		[]string{
			".sup counter",
			".sup counter increment mycount",
			".sup counter + mycount",
			".sup counter decrement mycount",
			".sup counter - mycount",
			".sup counter reset mycount",
			".sup counter list",
		},
		"utility",
	)
}

func (p *CounterPlugin) GetRequiredEnvVars() []string {
	return []string{}
}

func (p *CounterPlugin) Version() string {
	return "1.1.0"
}

func init() {
	plugin.RegisterPlugin(&CounterPlugin{})
}

func main() {}
