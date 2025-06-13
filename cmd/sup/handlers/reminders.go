package handlers

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"github.com/olebedev/when"
	"github.com/olebedev/when/rules/common"
	"github.com/olebedev/when/rules/en"
	"github.com/robfig/cron/v3"
	"github.com/rubiojr/sup/bot/handlers"
	"github.com/rubiojr/sup/cmd/sup/version"
	"github.com/rubiojr/sup/internal/client"
	"github.com/rubiojr/sup/internal/log"
	"github.com/rubiojr/sup/store"
)

type Reminder struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	RemindAt    time.Time `json:"remind_at"`
	CreatedAt   time.Time `json:"created_at"`
	Triggered   bool      `json:"triggered"`
	ChatID      string    `json:"chat_id"`
	CreatedBy   string    `json:"created_by"`
}

type RemindersHandler struct {
	store  store.Store
	parser *when.Parser
	cron   *cron.Cron
}

func NewRemindersHandler(store store.Store) *RemindersHandler {
	w := when.New(nil)
	w.Add(en.All...)
	w.Add(common.All...)

	handler := &RemindersHandler{
		store:  store,
		parser: w,
		cron:   cron.New(cron.WithSeconds()),
	}

	handler.cron.AddFunc("*/10 * * * * *", handler.checkAllReminders)
	handler.cron.Start()

	return handler
}

func (h *RemindersHandler) Name() string {
	return "rem"
}

func (h *RemindersHandler) Topics() []string {
	return []string{"rem", "reminder", "reminders"}
}

func (h *RemindersHandler) HandleMessage(msg *events.Message) error {
	var messageText string
	if msg.Message.GetConversation() != "" {
		messageText = msg.Message.GetConversation()
	} else if msg.Message.GetExtendedTextMessage() != nil {
		messageText = msg.Message.GetExtendedTextMessage().GetText()
	}

	parts := strings.Fields(messageText)
	var text string
	if len(parts) > 2 {
		text = strings.Join(parts[2:], " ")
	}

	c, err := client.GetClient()
	if err != nil {
		return fmt.Errorf("error getting client: %w", err)
	}

	sender := msg.Info.Sender.String()
	chatID := msg.Info.Chat.String()
	isGroup := msg.Info.Chat.Server == types.GroupServer

	if text == "" {
		return h.listReminders(c, msg, sender, chatID, isGroup)
	}

	args := strings.Fields(text)
	action := args[0]

	switch action {
	case "list", "ls":
		return h.listReminders(c, msg, sender, chatID, isGroup)
	case "delete", "del", "rm":
		if len(args) < 2 {
			c.SendText(msg.Info.Chat, "❌ Please provide reminder ID to delete")
			return nil
		}
		return h.deleteReminder(c, msg, sender, chatID, isGroup, args[1])
	case "clear":
		return h.clearReminders(c, msg, sender, chatID, isGroup)
	case "check":
		return h.checkUserReminders(c, msg, sender, chatID, isGroup)
	default:
		return h.createReminder(c, msg, sender, chatID, text)
	}
}

func (h *RemindersHandler) createReminder(c *client.Client, msg *events.Message, sender, chatID, message string) error {
	isGroup := msg.Info.Chat.Server == types.GroupServer
	reminderKey := h.getReminderKey(sender, chatID, isGroup)

	// Split on @ to separate description from time
	parts := strings.SplitN(message, "@", 2)
	if len(parts) < 2 {
		c.SendText(msg.Info.Chat, "❌ Please use format: description @ time\nExample: Call mom @ tomorrow 3pm")
		return nil
	}

	description := strings.TrimSpace(parts[0])
	timeStr := strings.TrimSpace(parts[1])

	if description == "" {
		c.SendText(msg.Info.Chat, "❌ Please provide a description for the reminder")
		return nil
	}

	if timeStr == "" {
		c.SendText(msg.Info.Chat, "❌ Please provide a time for the reminder")
		return nil
	}

	result, err := h.parser.Parse(timeStr, time.Now())
	if err != nil || result == nil {
		c.SendText(msg.Info.Chat, fmt.Sprintf("❌ Could not parse time '%s'. Try formats like 'tomorrow 3pm', 'in 2 hours', 'next friday'", timeStr))
		return nil
	}

	if result.Time.Before(time.Now()) {
		c.SendText(msg.Info.Chat, "❌ Cannot create reminders in the past")
		return nil
	}

	reminder := Reminder{
		ID:          fmt.Sprintf("%d", time.Now().UnixNano()),
		Description: description,
		RemindAt:    result.Time,
		CreatedAt:   time.Now(),
		Triggered:   false,
		ChatID:      chatID,
		CreatedBy:   sender,
	}

	log.Debug("Creating reminder", "user", sender, "chatID", chatID, "description", description, "remindAt", result.Time)

	// Validate that we have a valid chat ID
	if chatID == "" {
		log.Error("Chat ID is empty when creating reminder", "user", sender)
		c.SendText(msg.Info.Chat, "❌ Failed to save reminder: invalid chat ID")
		return nil
	}

	if err := h.saveReminder(reminderKey, reminder); err != nil {
		c.SendText(msg.Info.Chat, "❌ Failed to save reminder: "+err.Error())
		return nil
	}

	timeStr = result.Time.Format("Monday, January 2, 2006 at 3:04 PM")
	c.SendText(msg.Info.Chat, fmt.Sprintf("✅ Reminder set for %s: %s", timeStr, description))

	return nil
}

func (h *RemindersHandler) listReminders(c *client.Client, msg *events.Message, sender, chatID string, isGroup bool) error {
	// Extract phone number from sender (remove device ID part)
	senderPhone := h.extractPhoneNumber(sender)
	chatPhone := h.extractPhoneNumber(chatID)
	isOwnChat := senderPhone == chatPhone && !isGroup

	log.Debug("listReminders called", "sender", sender, "chatID", chatID, "isGroup", isGroup, "senderPhone", senderPhone, "chatPhone", chatPhone, "isOwnChat", isOwnChat)

	var reminders []Reminder
	var err error

	if isGroup {
		reminderKey := h.getReminderKey(sender, chatID, isGroup)
		reminders, err = h.getReminders(reminderKey)
		if err != nil {
			c.SendText(msg.Info.Chat, "❌ Failed to get reminders: "+err.Error())
			return nil
		}
	} else {
		// For non-group (private) chats, check if sender and chatID match
		if isOwnChat {
			// User's own chat - list all reminders from all chats for this user
			reminders, err = h.getAllUserReminders(sender)
			if err != nil {
				c.SendText(msg.Info.Chat, "❌ Failed to get reminders: "+err.Error())
				return nil
			}
		} else {
			// Different chat - list only reminders for this specific chat
			reminderKey := h.getReminderKey(sender, chatID, isGroup)
			reminders, err = h.getReminders(reminderKey)
			if err != nil {
				c.SendText(msg.Info.Chat, "❌ Failed to get reminders: "+err.Error())
				return nil
			}
		}
	}

	if len(reminders) == 0 {
		if isGroup {
			c.SendText(msg.Info.Chat, "📝 No active reminders for this group")
		} else if isOwnChat {
			c.SendText(msg.Info.Chat, "📝 No active reminders across all chats")
		} else {
			c.SendText(msg.Info.Chat, "📝 No active reminders")
		}
		return nil
	}

	sort.Slice(reminders, func(i, j int) bool {
		return reminders[i].RemindAt.Before(reminders[j].RemindAt)
	})

	var result strings.Builder
	if isGroup {
		result.WriteString(fmt.Sprintf("📝 Group reminders (%d):\n", len(reminders)))
	} else if isOwnChat {
		result.WriteString(fmt.Sprintf("📝 All your reminders (%d):\n", len(reminders)))
	} else {
		result.WriteString(fmt.Sprintf("📝 Active reminders (%d):\n", len(reminders)))
	}

	for _, reminder := range reminders {
		timeStr := reminder.RemindAt.Format("Mon Jan 2, 3:04 PM")
		if reminder.Description != "" {
			if isOwnChat {
				// Show chat context for user's own chat listing all reminders
				chatInfo := h.getChatInfo(reminder.ChatID)
				result.WriteString(fmt.Sprintf("• %s [%s]: %s %s\n", timeStr, reminder.ID[:8], reminder.Description, chatInfo))
			} else {
				result.WriteString(fmt.Sprintf("• %s [%s]: %s\n", timeStr, reminder.ID[:8], reminder.Description))
			}
		} else {
			if isOwnChat {
				chatInfo := h.getChatInfo(reminder.ChatID)
				result.WriteString(fmt.Sprintf("• %s [%s] %s\n", timeStr, reminder.ID[:8], chatInfo))
			} else {
				result.WriteString(fmt.Sprintf("• %s [%s]\n", timeStr, reminder.ID[:8]))
			}
		}
	}

	c.SendText(msg.Info.Chat, strings.TrimSpace(result.String()))
	return nil
}

func (h *RemindersHandler) deleteReminder(c *client.Client, msg *events.Message, sender, chatID string, isGroup bool, reminderID string) error {
	reminderKey := h.getReminderKey(sender, chatID, isGroup)
	reminders, err := h.getReminders(reminderKey)
	if err != nil {
		c.SendText(msg.Info.Chat, "❌ Failed to get reminders: "+err.Error())
		return nil
	}

	var filteredReminders []Reminder
	var deleted bool

	for _, reminder := range reminders {
		if strings.HasPrefix(reminder.ID, reminderID) {
			deleted = true
			continue
		}
		filteredReminders = append(filteredReminders, reminder)
	}

	if !deleted {
		c.SendText(msg.Info.Chat, "❌ Reminder not found")
		return nil
	}

	if err := h.saveReminders(reminderKey, filteredReminders); err != nil {
		c.SendText(msg.Info.Chat, "❌ Failed to delete reminder: "+err.Error())
		return nil
	}

	c.SendText(msg.Info.Chat, "✅ Reminder deleted")
	return nil
}

func (h *RemindersHandler) clearReminders(c *client.Client, msg *events.Message, sender, chatID string, isGroup bool) error {
	reminderKey := h.getReminderKey(sender, chatID, isGroup)
	if err := h.saveReminders(reminderKey, []Reminder{}); err != nil {
		c.SendText(msg.Info.Chat, "❌ Failed to clear reminders: "+err.Error())
		return nil
	}

	if isGroup {
		c.SendText(msg.Info.Chat, "✅ All group reminders cleared")
	} else {
		c.SendText(msg.Info.Chat, "✅ All reminders cleared")
	}
	return nil
}

func (h *RemindersHandler) checkUserReminders(c *client.Client, msg *events.Message, sender, chatID string, isGroup bool) error {
	reminderKey := h.getReminderKey(sender, chatID, isGroup)
	reminders, err := h.getReminders(reminderKey)
	if err != nil {
		c.SendText(msg.Info.Chat, "❌ Failed to get reminders: "+err.Error())
		return nil
	}

	now := time.Now()
	var dueReminders []Reminder
	var updatedReminders []Reminder

	for _, reminder := range reminders {
		if !reminder.Triggered && reminder.RemindAt.Before(now.Add(time.Minute)) {
			reminder.Triggered = true
			dueReminders = append(dueReminders, reminder)
		}
		updatedReminders = append(updatedReminders, reminder)
	}

	if len(dueReminders) == 0 {
		c.SendText(msg.Info.Chat, "📝 No reminders due")
		return nil
	}

	if err := h.saveReminders(reminderKey, updatedReminders); err != nil {
		c.SendText(msg.Info.Chat, "❌ Failed to update reminders: "+err.Error())
		return nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("🔔 %d reminder(s) due:\n", len(dueReminders)))

	for _, reminder := range dueReminders {
		if reminder.Description != "" {
			result.WriteString(fmt.Sprintf("• %s\n", reminder.Description))
		} else {
			result.WriteString(fmt.Sprintf("• Reminder [%s]\n", reminder.ID[:8]))
		}
	}

	log.Debug("Manual reminder check completed", "reminderKey", reminderKey, "dueCount", len(dueReminders))
	c.SendText(msg.Info.Chat, strings.TrimSpace(result.String()))
	return nil
}

func (h *RemindersHandler) checkAllReminders() {
	log.Debug("Checking all reminders and running garbage collection")

	c, err := client.GetClient()
	if err != nil {
		log.Error("Failed to get client for reminder check", "error", err)
		return
	}

	reminderKeys, err := h.getAllReminderKeys()
	if err != nil {
		log.Error("Failed to get reminder keys", "error", err)
		return
	}

	for _, reminderKey := range reminderKeys {
		// Run garbage collection for each key
		h.garbageCollect(reminderKey)
		// Check and notify for due reminders
		h.checkAndNotifyUser(c, reminderKey)
	}
}

func (h *RemindersHandler) checkAndNotifyUser(c *client.Client, reminderKey string) {
	reminders, err := h.getReminders(reminderKey)
	if err != nil {
		log.Error("Failed to get reminders for key", "reminderKey", reminderKey, "error", err)
		return
	}

	now := time.Now()
	var dueReminders []Reminder
	var updatedReminders []Reminder
	var hasUpdates bool

	for _, reminder := range reminders {
		if !reminder.Triggered && reminder.RemindAt.Before(now) {
			reminder.Triggered = true
			dueReminders = append(dueReminders, reminder)
			hasUpdates = true
		}
		updatedReminders = append(updatedReminders, reminder)
	}

	if len(dueReminders) == 0 {
		return
	}

	if hasUpdates {
		if err := h.saveReminders(reminderKey, updatedReminders); err != nil {
			log.Error("Failed to update reminders for key", "reminderKey", reminderKey, "error", err)
			return
		}
	}

	for _, reminder := range dueReminders {
		chatID := reminder.ChatID
		if chatID == "" {
			log.Warn("Reminder has empty chat ID, skipping", "reminderID", reminder.ID, "reminderKey", reminderKey)
			continue
		}

		var message string
		if reminder.Description != "" {
			message = fmt.Sprintf("🔔 Reminder: %s", reminder.Description)
		} else {
			message = fmt.Sprintf("🔔 Reminder [%s]", reminder.ID[:8])
		}

		log.Debug("Sending reminder", "reminderKey", reminderKey, "chatID", chatID, "description", reminder.Description, "reminderID", reminder.ID[:8])

		chat, err := types.ParseJID(chatID)
		if err != nil {
			log.Error("Failed to parse chat ID", "chatID", chatID, "reminderID", reminder.ID, "error", err)
			continue
		}

		err = c.SendText(chat, message)
		if err != nil {
			log.Error("Failed to send reminder", "chatID", chatID, "reminderID", reminder.ID, "error", err)
		} else {
			log.Info("Sent reminder", "chatID", chatID, "description", reminder.Description, "reminderID", reminder.ID[:8])
		}
	}
}

func (h *RemindersHandler) getReminderKey(sender, chatID string, isGroup bool) string {
	if isGroup {
		return fmt.Sprintf("group:%s", chatID)
	}
	return sender
}

func (h *RemindersHandler) getReminders(reminderKey string) ([]Reminder, error) {
	storeKey := fmt.Sprintf("%s:reminders", reminderKey)
	data, err := h.store.Get([]byte(storeKey))
	if err != nil || data == nil {
		return []Reminder{}, nil
	}

	var reminders []Reminder
	if err := json.Unmarshal(data, &reminders); err != nil {
		return nil, fmt.Errorf("failed to unmarshal reminders: %w", err)
	}

	return reminders, nil
}

func (h *RemindersHandler) saveReminder(reminderKey string, reminder Reminder) error {
	reminders, err := h.getReminders(reminderKey)
	if err != nil {
		return err
	}

	reminders = append(reminders, reminder)
	err = h.saveReminders(reminderKey, reminders)
	if err != nil {
		return err
	}

	// Add key to the index
	return h.addKeyToIndex(reminderKey)
}

func (h *RemindersHandler) saveReminders(reminderKey string, reminders []Reminder) error {
	storeKey := fmt.Sprintf("%s:reminders", reminderKey)
	data, err := json.Marshal(reminders)
	if err != nil {
		return fmt.Errorf("failed to marshal reminders: %w", err)
	}

	return h.store.Put([]byte(storeKey), data)
}

func (h *RemindersHandler) garbageCollect(reminderKey string) {
	reminders, err := h.getReminders(reminderKey)
	if err != nil {
		return
	}

	now := time.Now()
	cutoffTime := now.Add(-10 * time.Second) // Only remove reminders older than 10 seconds
	var activeReminders []Reminder
	removedCount := 0

	for _, reminder := range reminders {
		// Keep reminders that are either:
		// 1. In the future (not yet due)
		// 2. Past but within 10 seconds (recently triggered)
		if reminder.RemindAt.After(now) || reminder.RemindAt.After(cutoffTime) {
			activeReminders = append(activeReminders, reminder)
		} else {
			removedCount++
			log.Debug("Garbage collecting old reminder", "reminderKey", reminderKey, "reminderID", reminder.ID, "remindAt", reminder.RemindAt, "triggered", reminder.Triggered, "age", now.Sub(reminder.RemindAt))
		}
	}

	if len(activeReminders) != len(reminders) {
		log.Debug("Garbage collection completed", "reminderKey", reminderKey, "removed", removedCount, "remaining", len(activeReminders))
		h.saveReminders(reminderKey, activeReminders)

		// If no reminders left, remove key from index
		if len(activeReminders) == 0 {
			h.removeKeyFromIndex(reminderKey)
		}
	}
}

func (h *RemindersHandler) getAllReminderKeys() ([]string, error) {
	data, err := h.store.Get([]byte("reminder_keys_index"))
	if err != nil || data == nil {
		return []string{}, nil
	}

	var keys []string
	if err := json.Unmarshal(data, &keys); err != nil {
		return nil, fmt.Errorf("failed to unmarshal reminder keys index: %w", err)
	}

	return keys, nil
}

func (h *RemindersHandler) addKeyToIndex(reminderKey string) error {
	keys, err := h.getAllReminderKeys()
	if err != nil {
		return err
	}

	// Check if key already exists
	for _, existingKey := range keys {
		if existingKey == reminderKey {
			return nil
		}
	}

	// Add new key
	keys = append(keys, reminderKey)
	data, err := json.Marshal(keys)
	if err != nil {
		return fmt.Errorf("failed to marshal reminder keys index: %w", err)
	}

	return h.store.Put([]byte("reminder_keys_index"), data)
}

func (h *RemindersHandler) removeKeyFromIndex(reminderKey string) error {
	keys, err := h.getAllReminderKeys()
	if err != nil {
		return err
	}

	// Remove key from list
	var filteredKeys []string
	for _, existingKey := range keys {
		if existingKey != reminderKey {
			filteredKeys = append(filteredKeys, existingKey)
		}
	}

	data, err := json.Marshal(filteredKeys)
	if err != nil {
		return fmt.Errorf("failed to marshal reminder keys index: %w", err)
	}

	return h.store.Put([]byte("reminder_keys_index"), data)
}

func (h *RemindersHandler) getAllUserReminders(userID string) ([]Reminder, error) {
	log.Debug("Getting all user reminders", "userID", userID)

	allKeys, err := h.getAllReminderKeys()
	if err != nil {
		log.Error("Failed to get all reminder keys", "error", err)
		return nil, err
	}

	log.Debug("Found reminder keys", "keys", allKeys, "count", len(allKeys))

	var allReminders []Reminder

	for _, key := range allKeys {
		log.Debug("Processing key", "key", key, "userID", userID)

		h.garbageCollect(key)
		reminders, err := h.getReminders(key)
		if err != nil {
			log.Error("Failed to get reminders for key", "key", key, "error", err)
			continue
		}

		log.Debug("Found reminders for key", "key", key, "count", len(reminders))

		// Filter reminders created by this user
		for _, reminder := range reminders {
			if reminder.CreatedBy == userID {
				allReminders = append(allReminders, reminder)
				log.Debug("Added user reminder", "key", key, "reminderID", reminder.ID[:8])
			}
		}
	}

	log.Debug("Total reminders collected", "count", len(allReminders))
	return allReminders, nil
}

func (h *RemindersHandler) extractPhoneNumber(jid string) string {
	// Extract phone number from JID, handling device ID
	// Format: phone:device@server or phone@server
	if strings.Contains(jid, "@") {
		userPart := strings.Split(jid, "@")[0]
		if strings.Contains(userPart, ":") {
			return strings.Split(userPart, ":")[0]
		}
		return userPart
	}
	return jid
}

func (h *RemindersHandler) getChatInfo(chatID string) string {
	if strings.Contains(chatID, "@g.us") {
		return "(group)"
	}
	return "(private)"
}

func (h *RemindersHandler) GetHelp() handlers.HandlerHelp {
	return handlers.HandlerHelp{
		Name:        "reminders",
		Description: "Create and manage active time-based reminders with natural language",
		Usage:       ".sup rem <description> @ <time>",
		Examples: []string{
			".sup rem Meeting with John @ tomorrow 3pm",
			".sup rem Call mom @ in 2 hours",
			".sup rem Check the oven @ in 30 seconds",
			".sup rem Test reminder @ in 10 seconds",
			".sup rem list",
			".sup rem delete 12345678",
			".sup rem clear",
		},
		Category: "utility",
	}
}

func (h *RemindersHandler) Version() string {
	return version.String
}
