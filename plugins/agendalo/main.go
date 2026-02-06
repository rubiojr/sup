package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	pdk "github.com/extism/go-pdk"
	"github.com/rubiojr/sup/pkg/plugin"
)

const rateLimitWindow = time.Hour

type AgendaloPlugin struct{}

// AgendaEvent is a single event returned by the external command.
type AgendaEvent struct {
	Name string `json:"name"`
	Date string `json:"date"`
}

// AgendaCommandOutput is the JSON returned by the external command.
type AgendaCommandOutput struct {
	Events []AgendaEvent `json:"events"`
}

// StoredEvent is persisted in the store per sender.
type StoredEvent struct {
	Name      string `json:"name"`
	Date      string `json:"date"`
	CreatedAt int64  `json:"created_at"`
}

// rateLimitData tracks timestamps of add operations per sender.
type rateLimitData struct {
	Timestamps []int64 `json:"timestamps"`
}

func (p *AgendaloPlugin) Name() string {
	return "agendalo"
}

func (p *AgendaloPlugin) Topics() []string {
	return []string{"agendalo", "agenda"}
}

func (p *AgendaloPlugin) HandleMessage(input plugin.Input) plugin.Output {
	text := strings.TrimSpace(input.Message)
	sender := input.Sender
	isGroup := input.Info.IsGroup

	pdk.Log(pdk.LogInfo, fmt.Sprintf("received message from %s: %q", sender, text))

	storeKey := agendaKey(sender, isGroup)

	// Store sender name for CLI display
	if input.Info.PushName != "" {
		plugin.Storage().Set(fmt.Sprintf("name:%s", sender), []byte(input.Info.PushName))
	}

	if text == "" || text == "list" || text == "ls" {
		return listEvents(storeKey)
	}

	if text == "clear" {
		return clearEvents(storeKey)
	}

	// Rate limit check
	if !checkRateLimit(sender) {
		return plugin.Success("⏳ Rate limit reached. Please try again later.")
	}

	return updateAgenda(storeKey, text)
}

func agendaKey(sender string, isGroup bool) string {
	return fmt.Sprintf("agenda:%s", sender)
}

func checkRateLimit(sender string) bool {
	rlKey := fmt.Sprintf("ratelimit:%s", sender)
	store := plugin.Storage()

	maxPerHour := 5
	if v := os.Getenv("AGENDALO_RATE_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxPerHour = n
		}
	}

	var rl rateLimitData
	data, err := store.Get(rlKey)
	if err == nil && data != nil {
		json.Unmarshal(data, &rl)
	}

	now := time.Now().Unix()
	cutoff := now - int64(rateLimitWindow.Seconds())

	// Prune old
	valid := rl.Timestamps[:0]
	for _, ts := range rl.Timestamps {
		if ts > cutoff {
			valid = append(valid, ts)
		}
	}
	rl.Timestamps = valid

	if len(rl.Timestamps) >= maxPerHour {
		return false
	}

	rl.Timestamps = append(rl.Timestamps, now)
	if d, err := json.Marshal(rl); err == nil {
		store.Set(rlKey, d)
	}
	return true
}

func updateAgenda(storeKey, text string) plugin.Output {
	command := os.Getenv("AGENDALO_COMMAND")
	if command == "" {
		return plugin.Error("🚫 AGENDALO_COMMAND env var not set.")
	}

	// Build stdin with current agenda + user message
	existing := loadEvents(storeKey)
	var agendaJSON string
	if len(existing) > 0 {
		data, _ := json.Marshal(existing)
		agendaJSON = string(data)
	} else {
		agendaJSON = "[]"
	}

	stdin := fmt.Sprintf("<AGENDA>\n%s\n</AGENDA>\n<USER>\n%s\n</USER>", agendaJSON, text)

	pdk.Log(pdk.LogInfo, fmt.Sprintf("executing command: %s stdin: %q", command, stdin))
	resp, err := plugin.ExecCommand(command, stdin)
	if err != nil {
		return plugin.Error(fmt.Sprintf("🚫 Failed to run command: %v", err))
	}
	if !resp.Success {
		return plugin.Error(fmt.Sprintf("🚫 Command failed: %s", resp.Stderr))
	}

	stdout := strings.TrimSpace(resp.Stdout)
	// Extract JSON from the first markdown code fence if present,
	// ignoring any surrounding text the LLM may have added.
	if idx := strings.Index(stdout, "```"); idx >= 0 {
		rest := stdout[idx+3:]
		// Skip optional language tag (e.g. "json")
		if nl := strings.Index(rest, "\n"); nl >= 0 {
			rest = rest[nl+1:]
		}
		if end := strings.Index(rest, "```"); end >= 0 {
			stdout = strings.TrimSpace(rest[:end])
		}
	}

	var output AgendaCommandOutput
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		return plugin.Error(fmt.Sprintf("🚫 Failed to parse command output: %v\nstdout: %s\nstderr: %s", err, resp.Stdout, resp.Stderr))
	}

	// Replace the entire agenda
	now := time.Now().Unix()
	var updated []StoredEvent
	for _, evt := range output.Events {
		updated = append(updated, StoredEvent{
			Name:      evt.Name,
			Date:      evt.Date,
			CreatedAt: now,
		})
	}

	if err := saveEvents(storeKey, updated); err != nil {
		return plugin.Error("🚫 Failed to save events.")
	}

	if len(updated) == 0 {
		return plugin.Success("📅 Agenda updated (no events).")
	}

	return plugin.Success(fmt.Sprintf("📅 Agenda updated! %d event(s).", len(updated)))
}

func listEvents(storeKey string) plugin.Output {
	events := loadEvents(storeKey)
	now := time.Now()

	var upcoming []StoredEvent
	for _, evt := range events {
		t, err := parseEventDate(evt.Date)
		if err != nil || !t.Before(now) {
			upcoming = append(upcoming, evt)
		}
	}

	// Persist pruned list
	if len(upcoming) != len(events) {
		saveEvents(storeKey, upcoming)
	}

	if len(upcoming) == 0 {
		return plugin.Success("📅 No upcoming events in your agenda.")
	}

	var sb strings.Builder
	sb.WriteString("📅 Your agenda:\n")
	for i, evt := range upcoming {
		dateStr := evt.Date
		if t, err := parseEventDate(evt.Date); err == nil {
			dateStr = t.Format("Mon 02 Jan 2006 15:04")
		}
		sb.WriteString(fmt.Sprintf("%d. %s — %s\n", i+1, evt.Name, dateStr))
	}

	return plugin.Success(sb.String())
}

func parseEventDate(s string) (time.Time, error) {
	// Try ISO 8601 without timezone first (what sup-agenda returns)
	if t, err := time.ParseInLocation("2006-01-02T15:04:05", s, time.Local); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

func clearEvents(storeKey string) plugin.Output {
	if err := saveEvents(storeKey, nil); err != nil {
		return plugin.Error("🚫 Failed to clear agenda.")
	}
	return plugin.Success("🗑️ Agenda cleared.")
}

func loadEvents(key string) []StoredEvent {
	data, err := plugin.Storage().Get(key)
	if err != nil || data == nil {
		return nil
	}

	var events []StoredEvent
	if err := json.Unmarshal(data, &events); err != nil {
		return nil
	}
	return events
}

func saveEvents(key string, events []StoredEvent) error {
	if len(events) == 0 {
		return plugin.Storage().Set(key, []byte("[]"))
	}
	data, err := json.Marshal(events)
	if err != nil {
		return err
	}
	return plugin.Storage().Set(key, data)
}

func (p *AgendaloPlugin) GetHelp() plugin.HelpOutput {
	return plugin.NewHelpOutput(
		"agendalo",
		"Parse text with calendar dates and add them to your agenda",
		".sup agendalo <text with dates>",
		[]string{
			".sup agendalo dinner with Ana on Friday at 8pm",
			".sup agenda list",
			".sup agenda clear",
		},
		"utility",
	)
}

func (p *AgendaloPlugin) GetRequiredEnvVars() []string {
	return []string{"AGENDALO_COMMAND", "AGENDALO_RATE_LIMIT"}
}

func (p *AgendaloPlugin) Version() string {
	return "0.1.0"
}

func (p *AgendaloPlugin) HandleCLI(input plugin.CLIInput) plugin.CLIOutput {
	args := input.Args
	cmd := "list"
	if len(args) > 0 {
		cmd = args[0]
	}

	switch cmd {
	case "list", "ls":
		showAll := false
		sender := ""
		if len(args) > 1 {
			for _, a := range args[1:] {
				if a == "--all" || a == "-a" {
					showAll = true
				} else if sender == "" {
					sender = a
				}
			}
		}
		return p.cliList(sender, showAll)
	case "clear":
		if len(args) < 2 {
			return plugin.CLIOutput{Success: false, Error: "Usage: clear <sender>"}
		}
		return p.cliClear(args[1])
	default:
		return plugin.CLIOutput{Success: false, Error: fmt.Sprintf("Unknown command: %s\nAvailable: list [--all] [sender], clear <sender>", cmd)}
	}
}

func (p *AgendaloPlugin) cliList(sender string, showAll bool) plugin.CLIOutput {
	store := plugin.Storage()
	now := time.Now()

	if sender != "" {
		output := formatSenderAgenda(store, sender, now, showAll)
		return plugin.CLIOutput{Success: true, Output: output}
	}

	// List all agendas
	keys, err := store.List("agenda:")
	if err != nil {
		return plugin.CLIOutput{Success: false, Error: fmt.Sprintf("Failed to list keys: %v", err)}
	}

	if len(keys) == 0 {
		return plugin.CLIOutput{Success: true, Output: "No agendas found.\n"}
	}

	var sb strings.Builder
	for _, key := range keys {
		s := strings.TrimPrefix(key, "agenda:")
		label := s
		if name, err := store.Get(fmt.Sprintf("name:%s", s)); err == nil && name != nil && len(name) > 0 {
			label = fmt.Sprintf("%s (%s)", string(name), s)
		}
		sb.WriteString(fmt.Sprintf("── %s ──\n", label))
		sb.WriteString(formatSenderAgenda(store, s, now, showAll))
		sb.WriteString("\n")
	}

	return plugin.CLIOutput{Success: true, Output: sb.String()}
}

func formatSenderAgenda(store plugin.Store, sender string, now time.Time, showAll bool) string {
	key := fmt.Sprintf("agenda:%s", sender)
	data, err := store.Get(key)
	if err != nil || data == nil {
		return "  No events.\n"
	}

	var events []StoredEvent
	if err := json.Unmarshal(data, &events); err != nil {
		return "  Error reading events.\n"
	}

	var sb strings.Builder
	count := 0
	for _, evt := range events {
		t, err := parseEventDate(evt.Date)
		if !showAll && err == nil && t.Before(now) {
			continue
		}
		count++
		dateStr := evt.Date
		if err == nil {
			dateStr = t.Format("Mon 02 Jan 2006 15:04")
		}
		sb.WriteString(fmt.Sprintf("  %d. %s — %s\n", count, evt.Name, dateStr))
	}

	if count == 0 {
		return "  No upcoming events.\n"
	}
	return sb.String()
}

func (p *AgendaloPlugin) cliClear(sender string) plugin.CLIOutput {
	key := fmt.Sprintf("agenda:%s", sender)
	if err := saveEvents(key, nil); err != nil {
		return plugin.CLIOutput{Success: false, Error: fmt.Sprintf("Failed to clear: %v", err)}
	}
	return plugin.CLIOutput{Success: true, Output: fmt.Sprintf("Cleared agenda for %s\n", sender)}
}

func init() {
	plugin.RegisterPlugin(&AgendaloPlugin{})
}

func main() {}
