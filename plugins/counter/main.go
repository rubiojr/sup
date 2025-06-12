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

	if len(args) > 0 {
		action = args[0]
	}

	cacheKey := fmt.Sprintf("%s", input.Sender)
	switch action {
	case "increment", "inc", "+":
		return p.incrementCounter(cacheKey)
	case "decrement", "dec", "-":
		return p.decrementCounter(cacheKey)
	case "reset":
		return p.resetCounter(cacheKey)
	default:
		return p.getCounter(cacheKey)
	}
}

func (p *CounterPlugin) incrementCounter(cacheKey string) plugin.Output {
	count, err := p.getCurrentCount(cacheKey)
	if err != nil {
		count = 0
	}

	count++

	err = p.storeCount(cacheKey, count)
	if err != nil {
		return plugin.Error("Failed to store counter: " + err.Error())
	}

	return plugin.Success(fmt.Sprintf("Counter incremented to %d", count))
}

func (p *CounterPlugin) decrementCounter(cacheKey string) plugin.Output {
	count, err := p.getCurrentCount(cacheKey)
	if err != nil {
		count = 0
	}

	if count > 0 {
		count--
	}

	err = p.storeCount(cacheKey, count)
	if err != nil {
		return plugin.Error("Failed to store counter: " + err.Error())
	}

	return plugin.Success(fmt.Sprintf("Counter decremented to %d", count))
}

func (p *CounterPlugin) resetCounter(cacheKey string) plugin.Output {
	err := p.storeCount(cacheKey, 0)
	if err != nil {
		return plugin.Error("Failed to reset counter: " + err.Error())
	}

	return plugin.Success("Counter reset to 0")
}

func (p *CounterPlugin) getCounter(cacheKey string) plugin.Output {
	count, err := p.getCurrentCount(cacheKey)
	if err != nil {
		return plugin.Success("Counter value: 0 (not set)")
	}

	return plugin.Success(fmt.Sprintf("Counter value: %d", count))
}

func (p *CounterPlugin) getCurrentCount(cacheKey string) (int, error) {
	data, err := plugin.GetCache(cacheKey)
	if err != nil {
		return 0, err
	}
	if data == nil {
		return 0, fmt.Errorf("key not found in cache: %s", cacheKey)
	}

	count, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, fmt.Errorf("failed to parse counter value from cache: %w", err)
	}

	return count, nil
}

func (p *CounterPlugin) storeCount(cacheKey string, count int) error {
	countStr := strconv.Itoa(count)
	return plugin.SetCache(cacheKey, []byte(countStr))
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
		},
		"utility",
	)
}

func (p *CounterPlugin) GetRequiredEnvVars() []string {
	return []string{}
}

func (p *CounterPlugin) Version() string {
	return "1.0.0"
}

func init() {
	plugin.RegisterPlugin(&CounterPlugin{})
}

func main() {}
