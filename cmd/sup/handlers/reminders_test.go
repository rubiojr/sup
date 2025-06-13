package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/olebedev/when"
	"github.com/olebedev/when/rules/common"
	"github.com/olebedev/when/rules/en"
	"github.com/rubiojr/sup/store"
)

// mockStore implements the store.Store interface for testing
type mockStore struct {
	data map[string][]byte
}

func newMockStore() *mockStore {
	return &mockStore{
		data: make(map[string][]byte),
	}
}

func (m *mockStore) Get(key []byte) ([]byte, error) {
	value, exists := m.data[string(key)]
	if !exists {
		return nil, nil
	}
	return value, nil
}

func (m *mockStore) Put(key []byte, value []byte) error {
	m.data[string(key)] = value
	return nil
}

func (m *mockStore) Namespace(name string) store.Store {
	return &mockStore{
		data: m.data, // Share the same data for simplicity
	}
}

func TestRemindersHandler_CreateReminder(t *testing.T) {
	store := newMockStore()
	handler := &RemindersHandler{
		store: store,
	}

	// Initialize the when parser
	w := when.New(nil)
	w.Add(en.All...)
	w.Add(common.All...)
	handler.parser = w

	sender := "test@example.com"
	chatID := "test-chat@s.whatsapp.net"

	tests := []struct {
		name        string
		message     string
		expectError bool
		description string
	}{
		{
			name:        "valid reminder",
			message:     "Call mom @ in 1 hour",
			expectError: false,
			description: "Call mom",
		},
		{
			name:        "missing @ separator",
			message:     "Call mom in 1 hour",
			expectError: true,
			description: "",
		},
		{
			name:        "empty description",
			message:     " @ in 1 hour",
			expectError: true,
			description: "",
		},
		{
			name:        "empty time",
			message:     "Call mom @ ",
			expectError: true,
			description: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear store for each test
			store.data = make(map[string][]byte)

			// Parse the reminder creation logic
			parts := strings.SplitN(tt.message, "@", 2)
			if len(parts) < 2 {
				if !tt.expectError {
					t.Errorf("Expected no error but got missing @ separator")
				}
				return
			}

			description := strings.TrimSpace(parts[0])
			timeStr := strings.TrimSpace(parts[1])

			if description == "" {
				if !tt.expectError {
					t.Errorf("Expected no error but got empty description")
				}
				return
			}

			if timeStr == "" {
				if !tt.expectError {
					t.Errorf("Expected no error but got empty time")
				}
				return
			}

			result, err := handler.parser.Parse(timeStr, time.Now())
			if err != nil || result == nil {
				if !tt.expectError {
					t.Errorf("Expected no error but got time parsing error: %v", err)
				}
				return
			}

			if result.Time.Before(time.Now()) {
				if !tt.expectError {
					t.Errorf("Expected no error but got past time")
				}
				return
			}

			reminder := Reminder{
				ID:          "test-id",
				Description: description,
				RemindAt:    result.Time,
				CreatedAt:   time.Now(),
				Triggered:   false,
				ChatID:      chatID,
			}

			err = handler.saveReminder(sender, reminder)
			if err != nil {
				t.Errorf("Failed to save reminder: %v", err)
				return
			}

			// Verify reminder was saved
			reminders, err := handler.getReminders(sender)
			if err != nil {
				t.Errorf("Failed to get reminders: %v", err)
				return
			}

			if tt.expectError {
				t.Errorf("Expected error but reminder was created successfully")
				return
			}

			if len(reminders) != 1 {
				t.Errorf("Expected 1 reminder, got %d", len(reminders))
				return
			}

			savedReminder := reminders[0]
			if savedReminder.Description != tt.description {
				t.Errorf("Expected description '%s', got '%s'", tt.description, savedReminder.Description)
			}

			if savedReminder.ChatID != chatID {
				t.Errorf("Expected chat ID '%s', got '%s'", chatID, savedReminder.ChatID)
			}
		})
	}
}

func TestRemindersHandler_TimeFormatParsing(t *testing.T) {
	w := when.New(nil)
	w.Add(en.All...)
	w.Add(common.All...)

	testCases := []struct {
		input       string
		shouldParse bool
	}{
		{"in 1 hour", true},
		{"tomorrow 3pm", true},
		{"next friday", true},
		{"in 30 minutes", true},
		{"december 25 10am", true},
		{"invalid time format", false},
		{"", false},
		{"xyz", false},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result, err := w.Parse(tc.input, time.Now())

			if tc.shouldParse {
				if err != nil || result == nil {
					t.Errorf("Expected '%s' to parse successfully, but got error: %v", tc.input, err)
				}
			} else {
				if err == nil && result != nil {
					t.Errorf("Expected '%s' to fail parsing, but it succeeded", tc.input)
				}
			}
		})
	}
}

func TestRemindersHandler_KeyIndexManagement(t *testing.T) {
	store := newMockStore()
	handler := &RemindersHandler{
		store: store,
	}

	// Test adding keys to index
	keys := []string{"user1@example.com", "user2@example.com", "user3@example.com"}

	for _, key := range keys {
		err := handler.addKeyToIndex(key)
		if err != nil {
			t.Errorf("Failed to add key to index: %v", err)
		}
	}

	// Test getting all keys
	retrievedKeys, err := handler.getAllReminderKeys()
	if err != nil {
		t.Errorf("Failed to get reminder keys: %v", err)
	}

	if len(retrievedKeys) != len(keys) {
		t.Errorf("Expected %d keys, got %d", len(keys), len(retrievedKeys))
	}

	// Test removing a key
	err = handler.removeKeyFromIndex("user2@example.com")
	if err != nil {
		t.Errorf("Failed to remove key from index: %v", err)
	}

	retrievedKeys, err = handler.getAllReminderKeys()
	if err != nil {
		t.Errorf("Failed to get reminder keys after removal: %v", err)
	}

	if len(retrievedKeys) != 2 {
		t.Errorf("Expected 2 keys after removal, got %d", len(retrievedKeys))
	}

	// Verify the correct key was removed
	for _, key := range retrievedKeys {
		if key == "user2@example.com" {
			t.Errorf("Key 'user2@example.com' should have been removed but was still found")
		}
	}
}

func TestRemindersHandler_GarbageCollection(t *testing.T) {
	store := newMockStore()
	handler := &RemindersHandler{
		store: store,
	}

	reminderKey := "test@example.com"
	now := time.Now()

	// Create test reminders: one old triggered, one future, one recent triggered
	reminders := []Reminder{
		{
			ID:        "old-triggered",
			RemindAt:  now.Add(-25 * time.Hour), // 25 hours ago
			Triggered: true,
		},
		{
			ID:        "future",
			RemindAt:  now.Add(1 * time.Hour), // 1 hour from now
			Triggered: false,
		},
		{
			ID:        "recent-triggered",
			RemindAt:  now.Add(-1 * time.Hour), // 1 hour ago
			Triggered: true,
		},
	}

	// Save the reminders
	err := handler.saveReminders(reminderKey, reminders)
	if err != nil {
		t.Errorf("Failed to save reminders: %v", err)
	}

	// Run garbage collection
	handler.garbageCollect(reminderKey)

	// Get remaining reminders
	remaining, err := handler.getReminders(reminderKey)
	if err != nil {
		t.Errorf("Failed to get reminders after garbage collection: %v", err)
	}

	// Should have 2 reminders (future and recent-triggered)
	if len(remaining) != 2 {
		t.Errorf("Expected 2 reminders after garbage collection, got %d", len(remaining))
	}

	// Verify the old triggered reminder was removed
	for _, reminder := range remaining {
		if reminder.ID == "old-triggered" {
			t.Errorf("Old triggered reminder should have been garbage collected")
		}
	}
}

func TestReminder_JSONSerialization(t *testing.T) {
	now := time.Now()
	reminder := Reminder{
		ID:          "test-123",
		Description: "Test reminder",
		RemindAt:    now,
		CreatedAt:   now,
		Triggered:   false,
		ChatID:      "test@s.whatsapp.net",
	}

	// Test marshaling
	data, err := json.Marshal(reminder)
	if err != nil {
		t.Errorf("Failed to marshal reminder: %v", err)
	}

	// Test unmarshaling
	var unmarshaled Reminder
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Errorf("Failed to unmarshal reminder: %v", err)
	}

	// Verify fields
	if unmarshaled.ID != reminder.ID {
		t.Errorf("ID mismatch: expected %s, got %s", reminder.ID, unmarshaled.ID)
	}
	if unmarshaled.Description != reminder.Description {
		t.Errorf("Description mismatch: expected %s, got %s", reminder.Description, unmarshaled.Description)
	}
	if unmarshaled.ChatID != reminder.ChatID {
		t.Errorf("ChatID mismatch: expected %s, got %s", reminder.ChatID, unmarshaled.ChatID)
	}
	if unmarshaled.Triggered != reminder.Triggered {
		t.Errorf("Triggered mismatch: expected %v, got %v", reminder.Triggered, unmarshaled.Triggered)
	}
}

func TestRemindersHandler_GetReminderKey(t *testing.T) {
	handler := &RemindersHandler{}

	tests := []struct {
		name     string
		sender   string
		chatID   string
		isGroup  bool
		expected string
	}{
		{
			name:     "individual chat",
			sender:   "user1@example.com",
			chatID:   "user1@s.whatsapp.net",
			isGroup:  false,
			expected: "user1@example.com",
		},
		{
			name:     "group chat",
			sender:   "user1@example.com",
			chatID:   "group123@g.us",
			isGroup:  true,
			expected: "group:group123@g.us",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.getReminderKey(tt.sender, tt.chatID, tt.isGroup)
			if result != tt.expected {
				t.Errorf("Expected reminder key '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestRemindersHandler_GroupReminders(t *testing.T) {
	store := newMockStore()
	handler := &RemindersHandler{
		store: store,
	}

	// Initialize the when parser
	w := when.New(nil)
	w.Add(en.All...)
	w.Add(common.All...)
	handler.parser = w

	// Test data
	groupChatID := "group123@g.us"
	user1 := "user1@example.com"
	user2 := "user2@example.com"
	user3 := "user3@example.com"
	now := time.Now()

	// Create reminders from different users in the same group
	reminders := []struct {
		sender      string
		description string
		remindAt    time.Time
	}{
		{user1, "Team meeting", now.Add(1 * time.Hour)},
		{user2, "Project deadline", now.Add(2 * time.Hour)},
		{user3, "Coffee break", now.Add(30 * time.Minute)},
	}

	// Save reminders for the group
	groupKey := handler.getReminderKey("", groupChatID, true)
	var allReminders []Reminder
	for i, r := range reminders {
		reminder := Reminder{
			ID:          fmt.Sprintf("reminder-%d", i),
			Description: r.description,
			RemindAt:    r.remindAt,
			CreatedAt:   now,
			Triggered:   false,
			ChatID:      groupChatID,
		}
		allReminders = append(allReminders, reminder)
	}

	err := handler.saveReminders(groupKey, allReminders)
	if err != nil {
		t.Errorf("Failed to save group reminders: %v", err)
	}

	// Test that any user in the group can see all reminders
	for _, user := range []string{user1, user2, user3} {
		t.Run(fmt.Sprintf("user_%s_can_see_all_reminders", user), func(t *testing.T) {
			retrievedKey := handler.getReminderKey(user, groupChatID, true)
			retrievedReminders, err := handler.getReminders(retrievedKey)
			if err != nil {
				t.Errorf("Failed to get reminders for user %s: %v", user, err)
			}

			if len(retrievedReminders) != len(allReminders) {
				t.Errorf("Expected %d reminders, got %d for user %s", len(allReminders), len(retrievedReminders), user)
			}

			// Verify all reminders are accessible
			for _, expected := range allReminders {
				found := false
				for _, retrieved := range retrievedReminders {
					if retrieved.ID == expected.ID && retrieved.Description == expected.Description {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("User %s cannot see reminder with ID %s", user, expected.ID)
				}
			}
		})
	}
}

func TestRemindersHandler_GroupVsIndividualReminders(t *testing.T) {
	store := newMockStore()
	handler := &RemindersHandler{
		store: store,
	}

	// Initialize the when parser
	w := when.New(nil)
	w.Add(en.All...)
	w.Add(common.All...)
	handler.parser = w

	user1 := "user1@example.com"
	user2 := "user2@example.com"
	groupChatID := "group123@g.us"
	individualChatID := "user1@s.whatsapp.net"
	now := time.Now()

	// Create individual reminder for user1
	individualReminder := Reminder{
		ID:          "individual-1",
		Description: "Personal task",
		RemindAt:    now.Add(1 * time.Hour),
		CreatedAt:   now,
		Triggered:   false,
		ChatID:      individualChatID,
	}

	// Create group reminder
	groupReminder := Reminder{
		ID:          "group-1",
		Description: "Team task",
		RemindAt:    now.Add(2 * time.Hour),
		CreatedAt:   now,
		Triggered:   false,
		ChatID:      groupChatID,
	}

	// Save individual reminder
	individualKey := handler.getReminderKey(user1, individualChatID, false)
	err := handler.saveReminder(individualKey, individualReminder)
	if err != nil {
		t.Errorf("Failed to save individual reminder: %v", err)
	}

	// Save group reminder
	groupKey := handler.getReminderKey(user1, groupChatID, true)
	err = handler.saveReminder(groupKey, groupReminder)
	if err != nil {
		t.Errorf("Failed to save group reminder: %v", err)
	}

	// Test that individual reminders are isolated
	t.Run("individual_reminders_isolated", func(t *testing.T) {
		// User1 should see only their individual reminder in individual context
		individualReminders, err := handler.getReminders(individualKey)
		if err != nil {
			t.Errorf("Failed to get individual reminders: %v", err)
		}

		if len(individualReminders) != 1 {
			t.Errorf("Expected 1 individual reminder, got %d", len(individualReminders))
		}

		if individualReminders[0].ID != "individual-1" {
			t.Errorf("Expected individual reminder ID 'individual-1', got '%s'", individualReminders[0].ID)
		}

		// User2 should not see user1's individual reminders
		user2IndividualKey := handler.getReminderKey(user2, "user2@s.whatsapp.net", false)
		user2Reminders, err := handler.getReminders(user2IndividualKey)
		if err != nil {
			t.Errorf("Failed to get user2 individual reminders: %v", err)
		}

		if len(user2Reminders) != 0 {
			t.Errorf("Expected 0 individual reminders for user2, got %d", len(user2Reminders))
		}
	})

	// Test that group reminders are shared
	t.Run("group_reminders_shared", func(t *testing.T) {
		// Both users should see the same group reminder
		user1GroupKey := handler.getReminderKey(user1, groupChatID, true)
		user2GroupKey := handler.getReminderKey(user2, groupChatID, true)

		user1GroupReminders, err := handler.getReminders(user1GroupKey)
		if err != nil {
			t.Errorf("Failed to get user1 group reminders: %v", err)
		}

		user2GroupReminders, err := handler.getReminders(user2GroupKey)
		if err != nil {
			t.Errorf("Failed to get user2 group reminders: %v", err)
		}

		if len(user1GroupReminders) != 1 {
			t.Errorf("Expected 1 group reminder for user1, got %d", len(user1GroupReminders))
		}

		if len(user2GroupReminders) != 1 {
			t.Errorf("Expected 1 group reminder for user2, got %d", len(user2GroupReminders))
		}

		if user1GroupReminders[0].ID != user2GroupReminders[0].ID {
			t.Errorf("Group reminders should be the same for both users")
		}

		if user1GroupReminders[0].ID != "group-1" {
			t.Errorf("Expected group reminder ID 'group-1', got '%s'", user1GroupReminders[0].ID)
		}
	})
}

func TestRemindersHandler_GroupReminderDeletion(t *testing.T) {
	store := newMockStore()
	handler := &RemindersHandler{
		store: store,
	}

	// Initialize the when parser
	w := when.New(nil)
	w.Add(en.All...)
	w.Add(common.All...)
	handler.parser = w

	groupChatID := "group123@g.us"
	user1 := "user1@example.com"
	user2 := "user2@example.com"
	now := time.Now()

	// Create multiple group reminders
	reminders := []Reminder{
		{
			ID:          "group-reminder-1",
			Description: "First task",
			RemindAt:    now.Add(1 * time.Hour),
			CreatedAt:   now,
			Triggered:   false,
			ChatID:      groupChatID,
		},
		{
			ID:          "group-reminder-2",
			Description: "Second task",
			RemindAt:    now.Add(2 * time.Hour),
			CreatedAt:   now,
			Triggered:   false,
			ChatID:      groupChatID,
		},
	}

	// Save reminders
	groupKey := handler.getReminderKey(user1, groupChatID, true)
	err := handler.saveReminders(groupKey, reminders)
	if err != nil {
		t.Errorf("Failed to save group reminders: %v", err)
	}

	// Test that any user can delete group reminders
	t.Run("any_user_can_delete_group_reminders", func(t *testing.T) {
		// User2 deletes a reminder created by user1
		user2GroupKey := handler.getReminderKey(user2, groupChatID, true)

		// Get current reminders
		currentReminders, err := handler.getReminders(user2GroupKey)
		if err != nil {
			t.Errorf("Failed to get current reminders: %v", err)
		}

		if len(currentReminders) != 2 {
			t.Errorf("Expected 2 reminders before deletion, got %d", len(currentReminders))
		}

		// Delete first reminder
		var filteredReminders []Reminder
		for _, reminder := range currentReminders {
			if !strings.HasPrefix(reminder.ID, "group-reminder-1") {
				filteredReminders = append(filteredReminders, reminder)
			}
		}

		err = handler.saveReminders(user2GroupKey, filteredReminders)
		if err != nil {
			t.Errorf("Failed to save filtered reminders: %v", err)
		}

		// Verify deletion is visible to all users
		user1GroupKey := handler.getReminderKey(user1, groupChatID, true)
		user1Reminders, err := handler.getReminders(user1GroupKey)
		if err != nil {
			t.Errorf("Failed to get user1 reminders after deletion: %v", err)
		}

		if len(user1Reminders) != 1 {
			t.Errorf("Expected 1 reminder after deletion for user1, got %d", len(user1Reminders))
		}

		user2Reminders, err := handler.getReminders(user2GroupKey)
		if err != nil {
			t.Errorf("Failed to get user2 reminders after deletion: %v", err)
		}

		if len(user2Reminders) != 1 {
			t.Errorf("Expected 1 reminder after deletion for user2, got %d", len(user2Reminders))
		}

		// Verify the correct reminder was deleted
		if user1Reminders[0].ID != "group-reminder-2" {
			t.Errorf("Wrong reminder remained after deletion: expected 'group-reminder-2', got '%s'", user1Reminders[0].ID)
		}
	})
}

func TestRemindersHandler_GroupReminderKeyIndex(t *testing.T) {
	store := newMockStore()
	handler := &RemindersHandler{
		store: store,
	}

	// Test adding keys to index
	keys := []string{"user1@example.com", "group:group123@g.us", "user2@example.com"}

	for _, key := range keys {
		err := handler.addKeyToIndex(key)
		if err != nil {
			t.Errorf("Failed to add key to index: %v", err)
		}
	}

	// Test getting all keys
	retrievedKeys, err := handler.getAllReminderKeys()
	if err != nil {
		t.Errorf("Failed to get reminder keys: %v", err)
	}

	if len(retrievedKeys) != len(keys) {
		t.Errorf("Expected %d keys, got %d", len(keys), len(retrievedKeys))
	}

	// Test removing a key
	err = handler.removeKeyFromIndex("group:group123@g.us")
	if err != nil {
		t.Errorf("Failed to remove key from index: %v", err)
	}

	retrievedKeys, err = handler.getAllReminderKeys()
	if err != nil {
		t.Errorf("Failed to get reminder keys after removal: %v", err)
	}

	if len(retrievedKeys) != 2 {
		t.Errorf("Expected 2 keys after removal, got %d", len(retrievedKeys))
	}

	// Verify the correct key was removed
	for _, key := range retrievedKeys {
		if key == "group:group123@g.us" {
			t.Errorf("Group key should have been removed but was still found")
		}
	}
}
