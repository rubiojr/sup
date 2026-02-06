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

func (m *mockStore) Delete(key []byte) error {
	delete(m.data, string(key))
	return nil
}

func (m *mockStore) List(prefix string) ([]string, error) {
	var keys []string
	for k := range m.data {
		if len(prefix) == 0 || len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			keys = append(keys, k)
		}
	}
	return keys, nil
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
		{"in 10 seconds", true},
		{"in 30 seconds", true},
		{"in 1 second", true},
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

	// Create test reminders: one old, one future, one recent within cutoff
	reminders := []Reminder{
		{
			ID:        "old-triggered",
			RemindAt:  now.Add(-25 * time.Hour), // 25 hours ago - should be removed
			Triggered: true,
		},
		{
			ID:        "future",
			RemindAt:  now.Add(1 * time.Hour), // 1 hour from now - should be kept
			Triggered: false,
		},
		{
			ID:        "recent-triggered",
			RemindAt:  now.Add(-5 * time.Second), // 5 seconds ago - should be kept
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

	// Should have 2 reminders (future and recent-triggered within cutoff)
	if len(remaining) != 2 {
		t.Errorf("Expected 2 reminders after garbage collection, got %d", len(remaining))
	}

	// Verify correct reminders remain
	remainingIDs := make(map[string]bool)
	for _, reminder := range remaining {
		remainingIDs[reminder.ID] = true
	}

	if remainingIDs["old-triggered"] {
		t.Errorf("Old triggered reminder should have been garbage collected")
	}
	if !remainingIDs["future"] {
		t.Errorf("Future reminder should not have been garbage collected")
	}
	if !remainingIDs["recent-triggered"] {
		t.Errorf("Recent triggered reminder should not have been garbage collected")
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

func TestRemindersHandler_OldReminderGarbageCollection(t *testing.T) {
	store := newMockStore()
	handler := &RemindersHandler{
		store: store,
	}

	reminderKey := "test@example.com"
	now := time.Now()

	// Create test reminders with different states
	reminders := []Reminder{
		{
			ID:        "very-old",
			RemindAt:  now.Add(-25 * time.Hour), // Very old - should be removed
			Triggered: false,
		},
		{
			ID:        "old-beyond-cutoff",
			RemindAt:  now.Add(-30 * time.Second), // 30 seconds old - should be removed
			Triggered: true,
		},
		{
			ID:        "recent-within-cutoff",
			RemindAt:  now.Add(-5 * time.Second), // 5 seconds old - should be kept
			Triggered: true,
		},
		{
			ID:        "future-untriggered",
			RemindAt:  now.Add(1 * time.Hour), // Future - should be kept
			Triggered: false,
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

	// Should have 2 reminders (recent-within-cutoff and future-untriggered)
	if len(remaining) != 2 {
		t.Errorf("Expected 2 reminders after garbage collection, got %d", len(remaining))
		for _, r := range remaining {
			t.Logf("Remaining reminder: ID=%s, RemindAt=%s, Triggered=%v", r.ID, r.RemindAt.Format("2006-01-02 15:04:05"), r.Triggered)
		}
	}

	// Verify the correct reminders remain
	remainingIDs := make(map[string]bool)
	for _, reminder := range remaining {
		remainingIDs[reminder.ID] = true
	}

	// Old reminders should be removed
	if remainingIDs["very-old"] {
		t.Errorf("Very old reminder should have been garbage collected")
	}
	if remainingIDs["old-beyond-cutoff"] {
		t.Errorf("Old reminder beyond cutoff should have been garbage collected")
	}

	// Recent and future reminders should remain
	if !remainingIDs["recent-within-cutoff"] {
		t.Errorf("Recent reminder within cutoff should not have been garbage collected")
	}
	if !remainingIDs["future-untriggered"] {
		t.Errorf("Future untriggered reminder should not have been garbage collected")
	}
}

func TestRemindersHandler_ListFiltersOutOldReminders(t *testing.T) {
	store := newMockStore()
	handler := &RemindersHandler{
		store: store,
	}

	// Initialize the when parser
	w := when.New(nil)
	w.Add(en.All...)
	w.Add(common.All...)
	handler.parser = w

	reminderKey := "test@example.com"
	now := time.Now()

	// Create mix of old past, recent past, and future reminders
	reminders := []Reminder{
		{
			ID:          "old-past-1",
			Description: "Old meeting",
			RemindAt:    now.Add(-2 * time.Hour), // 2 hours ago - should be removed
			Triggered:   false,
		},
		{
			ID:          "old-past-2",
			Description: "Another old task",
			RemindAt:    now.Add(-30 * time.Second), // 30 seconds ago - should be removed
			Triggered:   false,
		},
		{
			ID:          "recent-past",
			Description: "Recent task",
			RemindAt:    now.Add(-5 * time.Second), // 5 seconds ago - should be kept
			Triggered:   true,
		},
		{
			ID:          "future-1",
			Description: "Upcoming meeting",
			RemindAt:    now.Add(1 * time.Hour), // 1 hour from now
			Triggered:   false,
		},
		{
			ID:          "future-2",
			Description: "Tomorrow task",
			RemindAt:    now.Add(24 * time.Hour), // 24 hours from now
			Triggered:   false,
		},
	}

	// Save all reminders
	err := handler.saveReminders(reminderKey, reminders)
	if err != nil {
		t.Errorf("Failed to save reminders: %v", err)
	}

	// Simulate getting reminders like listReminders would do
	handler.garbageCollect(reminderKey)
	remaining, err := handler.getReminders(reminderKey)
	if err != nil {
		t.Errorf("Failed to get reminders: %v", err)
	}

	// Should have 3 reminders (recent-past within cutoff + 2 future)
	if len(remaining) != 3 {
		t.Errorf("Expected 3 reminders after cleanup, got %d", len(remaining))
		for _, r := range remaining {
			t.Logf("Found reminder: ID=%s, RemindAt=%s, Triggered=%v", r.ID, r.RemindAt.Format("2006-01-02 15:04:05"), r.Triggered)
		}
	}

	// Verify old past reminders are removed
	remainingIDs := make(map[string]bool)
	for _, reminder := range remaining {
		remainingIDs[reminder.ID] = true
	}

	if remainingIDs["old-past-1"] {
		t.Errorf("Old past reminder 'old-past-1' should have been removed")
	}
	if remainingIDs["old-past-2"] {
		t.Errorf("Old past reminder 'old-past-2' should have been removed")
	}

	// Verify recent and future reminders are present
	if !remainingIDs["recent-past"] {
		t.Errorf("Recent past reminder 'recent-past' should be in the list")
	}
	if !remainingIDs["future-1"] {
		t.Errorf("Future reminder 'future-1' should be in the list")
	}
	if !remainingIDs["future-2"] {
		t.Errorf("Future reminder 'future-2' should be in the list")
	}
}

func TestRemindersHandler_SecondPrecisionReminders(t *testing.T) {
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
	chatID := "test@s.whatsapp.net"
	now := time.Now()

	// Test second-precision parsing
	testCases := []struct {
		timeStr  string
		expected time.Duration
	}{
		{"in 5 seconds", 5 * time.Second},
		{"in 10 seconds", 10 * time.Second},
		{"in 30 seconds", 30 * time.Second},
		{"in 1 second", 1 * time.Second},
	}

	for _, tc := range testCases {
		t.Run(tc.timeStr, func(t *testing.T) {
			// Parse the time
			result, err := handler.parser.Parse(tc.timeStr, now)
			if err != nil || result == nil {
				t.Errorf("Failed to parse '%s': %v", tc.timeStr, err)
				return
			}

			// Check if the parsed time is approximately correct (within 1 second tolerance)
			expectedTime := now.Add(tc.expected)
			diff := result.Time.Sub(expectedTime)
			if diff < -time.Second || diff > time.Second {
				t.Errorf("Time parsing for '%s' inaccurate: expected ~%s, got %s (diff: %s)",
					tc.timeStr, expectedTime.Format("15:04:05"), result.Time.Format("15:04:05"), diff)
			}

			// Test creating a reminder with second precision
			reminder := Reminder{
				ID:          fmt.Sprintf("test-%d", time.Now().UnixNano()),
				Description: fmt.Sprintf("Test reminder for %s", tc.timeStr),
				RemindAt:    result.Time,
				CreatedAt:   now,
				Triggered:   false,
				ChatID:      chatID,
			}

			err = handler.saveReminder(sender, reminder)
			if err != nil {
				t.Errorf("Failed to save second-precision reminder: %v", err)
				return
			}

			// Verify the reminder was saved with correct precision
			reminders, err := handler.getReminders(sender)
			if err != nil {
				t.Errorf("Failed to get reminders: %v", err)
				return
			}

			found := false
			for _, saved := range reminders {
				if saved.ID == reminder.ID {
					found = true
					// Check if the time was preserved with second precision
					if !saved.RemindAt.Equal(reminder.RemindAt) {
						t.Errorf("Reminder time precision lost: expected %s, got %s",
							reminder.RemindAt.Format("15:04:05.000"), saved.RemindAt.Format("15:04:05.000"))
					}
					break
				}
			}

			if !found {
				t.Errorf("Second-precision reminder not found after saving")
			}

			// Clean up for next test
			handler.saveReminders(sender, []Reminder{})
		})
	}
}

func TestRemindersHandler_SecondPrecisionTriggering(t *testing.T) {
	store := newMockStore()
	handler := &RemindersHandler{
		store: store,
	}

	// Initialize the when parser
	w := when.New(nil)
	w.Add(en.All...)
	w.Add(common.All...)
	handler.parser = w

	reminderKey := "test@example.com"
	chatID := "test@s.whatsapp.net"
	now := time.Now()

	// Create a reminder that should trigger in 2 seconds
	reminderTime := now.Add(2 * time.Second)
	reminder := Reminder{
		ID:          "second-test",
		Description: "Should trigger in 2 seconds",
		RemindAt:    reminderTime,
		CreatedAt:   now,
		Triggered:   false,
		ChatID:      chatID,
	}

	// Save the reminder
	err := handler.saveReminder(reminderKey, reminder)
	if err != nil {
		t.Errorf("Failed to save reminder: %v", err)
	}

	// Test that reminder is not yet due
	currentTime := now.Add(1 * time.Second)
	reminders, err := handler.getReminders(reminderKey)
	if err != nil {
		t.Errorf("Failed to get reminders: %v", err)
	}

	var dueReminders []Reminder
	for _, r := range reminders {
		if !r.Triggered && r.RemindAt.Before(currentTime) {
			dueReminders = append(dueReminders, r)
		}
	}

	if len(dueReminders) != 0 {
		t.Errorf("Reminder should not be due yet at +1 second, but found %d due reminders", len(dueReminders))
	}

	// Test that reminder is due after the trigger time
	currentTime = now.Add(3 * time.Second)
	dueReminders = nil
	for _, r := range reminders {
		if !r.Triggered && r.RemindAt.Before(currentTime) {
			dueReminders = append(dueReminders, r)
		}
	}

	if len(dueReminders) != 1 {
		t.Errorf("Expected 1 due reminder at +3 seconds, but found %d", len(dueReminders))
	}

	if len(dueReminders) > 0 && dueReminders[0].ID != "second-test" {
		t.Errorf("Wrong reminder triggered: expected 'second-test', got '%s'", dueReminders[0].ID)
	}

	// Verify the timing precision
	if len(dueReminders) > 0 {
		actualTriggerDelay := currentTime.Sub(dueReminders[0].RemindAt)
		if actualTriggerDelay < 0 || actualTriggerDelay > 2*time.Second {
			t.Errorf("Timing precision issue: reminder should trigger within 2 seconds of due time, but delay was %s", actualTriggerDelay)
		}
	}
}

func TestRemindersHandler_ExtractPhoneNumber(t *testing.T) {
	handler := &RemindersHandler{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "JID with device ID",
			input:    "34633010091:68@s.whatsapp.net",
			expected: "34633010091",
		},
		{
			name:     "JID without device ID",
			input:    "34633010091@s.whatsapp.net",
			expected: "34633010091",
		},
		{
			name:     "Group JID",
			input:    "120363123456789@g.us",
			expected: "120363123456789",
		},
		{
			name:     "Just phone number",
			input:    "34633010091",
			expected: "34633010091",
		},
		{
			name:     "Complex device ID",
			input:    "1234567890:99@s.whatsapp.net",
			expected: "1234567890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.extractPhoneNumber(tt.input)
			if result != tt.expected {
				t.Errorf("extractPhoneNumber(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRemindersHandler_CreatedByField(t *testing.T) {
	store := newMockStore()
	handler := &RemindersHandler{
		store: store,
	}

	w := when.New(nil)
	w.Add(en.All...)
	w.Add(common.All...)
	handler.parser = w

	sender := "user123@s.whatsapp.net"
	chatID := "user123@s.whatsapp.net"

	reminder := Reminder{
		ID:          "test-reminder-1",
		Description: "Test reminder",
		RemindAt:    time.Now().Add(time.Hour),
		CreatedAt:   time.Now(),
		Triggered:   false,
		ChatID:      chatID,
		CreatedBy:   sender,
	}

	err := handler.saveReminder(sender, reminder)
	if err != nil {
		t.Fatalf("Failed to save reminder: %v", err)
	}

	reminders, err := handler.getReminders(sender)
	if err != nil {
		t.Fatalf("Failed to get reminders: %v", err)
	}

	if len(reminders) != 1 {
		t.Fatalf("Expected 1 reminder, got %d", len(reminders))
	}

	if reminders[0].CreatedBy != sender {
		t.Errorf("Expected CreatedBy to be %q, got %q", sender, reminders[0].CreatedBy)
	}
}

func TestRemindersHandler_GroupNameField(t *testing.T) {
	store := newMockStore()
	handler := &RemindersHandler{
		store: store,
	}

	w := when.New(nil)
	w.Add(en.All...)
	w.Add(common.All...)
	handler.parser = w

	sender := "user123@s.whatsapp.net"
	groupChatID := "120363123456789@g.us"
	groupName := "Test Group"

	reminder := Reminder{
		ID:          "test-reminder-1",
		Description: "Test group reminder",
		RemindAt:    time.Now().Add(time.Hour),
		CreatedAt:   time.Now(),
		Triggered:   false,
		ChatID:      groupChatID,
		CreatedBy:   sender,
		GroupName:   groupName,
	}

	groupKey := fmt.Sprintf("group:%s", groupChatID)
	err := handler.saveReminder(groupKey, reminder)
	if err != nil {
		t.Fatalf("Failed to save group reminder: %v", err)
	}

	reminders, err := handler.getReminders(groupKey)
	if err != nil {
		t.Fatalf("Failed to get group reminders: %v", err)
	}

	if len(reminders) != 1 {
		t.Fatalf("Expected 1 reminder, got %d", len(reminders))
	}

	if reminders[0].GroupName != groupName {
		t.Errorf("Expected GroupName to be %q, got %q", groupName, reminders[0].GroupName)
	}

	if reminders[0].CreatedBy != sender {
		t.Errorf("Expected CreatedBy to be %q, got %q", sender, reminders[0].CreatedBy)
	}
}

func TestRemindersHandler_GetAllUserReminders(t *testing.T) {
	store := newMockStore()
	handler := &RemindersHandler{
		store: store,
	}

	w := when.New(nil)
	w.Add(en.All...)
	w.Add(common.All...)
	handler.parser = w

	user1 := "user1@s.whatsapp.net"
	user2 := "user2@s.whatsapp.net"
	groupChatID := "120363123456789@g.us"

	// Create reminders for user1 in private chat
	privateReminder := Reminder{
		ID:          "private-reminder",
		Description: "Private reminder",
		RemindAt:    time.Now().Add(time.Hour),
		CreatedAt:   time.Now(),
		Triggered:   false,
		ChatID:      user1,
		CreatedBy:   user1,
	}
	handler.saveReminder(user1, privateReminder)

	// Create reminders for user1 in group chat
	groupReminder := Reminder{
		ID:          "group-reminder",
		Description: "Group reminder",
		RemindAt:    time.Now().Add(time.Hour),
		CreatedAt:   time.Now(),
		Triggered:   false,
		ChatID:      groupChatID,
		CreatedBy:   user1,
		GroupName:   "Test Group",
	}
	groupKey := fmt.Sprintf("group:%s", groupChatID)
	handler.saveReminder(groupKey, groupReminder)

	// Create reminder for user2 in the same group
	user2GroupReminder := Reminder{
		ID:          "user2-group-reminder",
		Description: "User2 group reminder",
		RemindAt:    time.Now().Add(time.Hour),
		CreatedAt:   time.Now(),
		Triggered:   false,
		ChatID:      groupChatID,
		CreatedBy:   user2,
		GroupName:   "Test Group",
	}
	handler.saveReminder(groupKey, user2GroupReminder)

	// Get all reminders for user1
	user1Reminders, err := handler.getAllUserReminders(user1)
	if err != nil {
		t.Fatalf("Failed to get all user reminders: %v", err)
	}

	// Should get 2 reminders (private + group created by user1)
	if len(user1Reminders) != 2 {
		t.Fatalf("Expected 2 reminders for user1, got %d", len(user1Reminders))
	}

	// Verify all reminders belong to user1
	for _, reminder := range user1Reminders {
		if reminder.CreatedBy != user1 {
			t.Errorf("Expected all reminders to be created by %q, got reminder created by %q", user1, reminder.CreatedBy)
		}
	}

	// Verify we have both private and group reminders
	hasPrivate := false
	hasGroup := false
	for _, reminder := range user1Reminders {
		if reminder.ID == "private-reminder" {
			hasPrivate = true
		}
		if reminder.ID == "group-reminder" {
			hasGroup = true
		}
	}

	if !hasPrivate {
		t.Error("Expected to find private reminder")
	}
	if !hasGroup {
		t.Error("Expected to find group reminder")
	}
}

func TestRemindersHandler_GetChatInfo(t *testing.T) {
	handler := &RemindersHandler{}

	tests := []struct {
		name      string
		chatID    string
		groupName string
		expected  string
	}{
		{
			name:      "Group chat with name",
			chatID:    "120363123456789@g.us",
			groupName: "Family Group",
			expected:  "(group: Family Group)",
		},
		{
			name:      "Group chat without name",
			chatID:    "120363123456789@g.us",
			groupName: "",
			expected:  "(group)",
		},
		{
			name:      "Private chat",
			chatID:    "34633010091@s.whatsapp.net",
			groupName: "",
			expected:  "(private)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.getChatInfo(tt.chatID, tt.groupName)
			if result != tt.expected {
				t.Errorf("getChatInfo(%q, %q) = %q, want %q", tt.chatID, tt.groupName, result, tt.expected)
			}
		})
	}
}

func TestRemindersHandler_OwnChatDetection(t *testing.T) {
	store := newMockStore()
	handler := &RemindersHandler{
		store: store,
	}

	w := when.New(nil)
	w.Add(en.All...)
	w.Add(common.All...)
	handler.parser = w

	// Test data
	userPhone := "34633010091"
	senderWithDevice := "34633010091:68@s.whatsapp.net"
	chatWithoutDevice := "34633010091@s.whatsapp.net"

	// Create reminders in different contexts
	privateReminder := Reminder{
		ID:          "private-reminder",
		Description: "Private reminder",
		RemindAt:    time.Now().Add(time.Hour),
		CreatedAt:   time.Now(),
		Triggered:   false,
		ChatID:      chatWithoutDevice,
		CreatedBy:   senderWithDevice,
	}
	handler.saveReminder(senderWithDevice, privateReminder)

	groupReminder := Reminder{
		ID:          "group-reminder",
		Description: "Group reminder",
		RemindAt:    time.Now().Add(time.Hour),
		CreatedAt:   time.Now(),
		Triggered:   false,
		ChatID:      "120363123456789@g.us",
		CreatedBy:   senderWithDevice,
		GroupName:   "Test Group",
	}
	groupKey := "group:120363123456789@g.us"
	handler.saveReminder(groupKey, groupReminder)

	// Test phone number extraction
	extractedSender := handler.extractPhoneNumber(senderWithDevice)
	extractedChat := handler.extractPhoneNumber(chatWithoutDevice)

	if extractedSender != userPhone {
		t.Errorf("Expected extracted sender to be %q, got %q", userPhone, extractedSender)
	}

	if extractedChat != userPhone {
		t.Errorf("Expected extracted chat to be %q, got %q", userPhone, extractedChat)
	}

	// Test that own chat detection works (sender phone == chat phone)
	isOwnChat := extractedSender == extractedChat
	if !isOwnChat {
		t.Error("Expected own chat detection to work with device ID in sender")
	}

	// Test getAllUserReminders works correctly
	allReminders, err := handler.getAllUserReminders(senderWithDevice)
	if err != nil {
		t.Fatalf("Failed to get all user reminders: %v", err)
	}

	if len(allReminders) != 2 {
		t.Fatalf("Expected 2 reminders, got %d", len(allReminders))
	}

	// Verify both reminders are returned
	hasPrivate := false
	hasGroup := false
	for _, reminder := range allReminders {
		if reminder.ID == "private-reminder" {
			hasPrivate = true
		}
		if reminder.ID == "group-reminder" {
			hasGroup = true
		}
	}

	if !hasPrivate {
		t.Error("Expected to find private reminder")
	}
	if !hasGroup {
		t.Error("Expected to find group reminder")
	}
}

func TestRemindersHandler_JSONSerialization_WithNewFields(t *testing.T) {
	reminder := Reminder{
		ID:          "test-id",
		Description: "Test description",
		RemindAt:    time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
		CreatedAt:   time.Date(2023, 12, 31, 12, 0, 0, 0, time.UTC),
		Triggered:   false,
		ChatID:      "120363123456789@g.us",
		CreatedBy:   "user123@s.whatsapp.net",
		GroupName:   "Test Group Name",
	}

	// Test JSON marshaling
	data, err := json.Marshal(reminder)
	if err != nil {
		t.Fatalf("Failed to marshal reminder: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled Reminder
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal reminder: %v", err)
	}

	// Verify all fields are preserved
	if unmarshaled.ID != reminder.ID {
		t.Errorf("ID mismatch: expected %q, got %q", reminder.ID, unmarshaled.ID)
	}
	if unmarshaled.Description != reminder.Description {
		t.Errorf("Description mismatch: expected %q, got %q", reminder.Description, unmarshaled.Description)
	}
	if !unmarshaled.RemindAt.Equal(reminder.RemindAt) {
		t.Errorf("RemindAt mismatch: expected %v, got %v", reminder.RemindAt, unmarshaled.RemindAt)
	}
	if !unmarshaled.CreatedAt.Equal(reminder.CreatedAt) {
		t.Errorf("CreatedAt mismatch: expected %v, got %v", reminder.CreatedAt, unmarshaled.CreatedAt)
	}
	if unmarshaled.Triggered != reminder.Triggered {
		t.Errorf("Triggered mismatch: expected %v, got %v", reminder.Triggered, unmarshaled.Triggered)
	}
	if unmarshaled.ChatID != reminder.ChatID {
		t.Errorf("ChatID mismatch: expected %q, got %q", reminder.ChatID, unmarshaled.ChatID)
	}
	if unmarshaled.CreatedBy != reminder.CreatedBy {
		t.Errorf("CreatedBy mismatch: expected %q, got %q", reminder.CreatedBy, unmarshaled.CreatedBy)
	}
	if unmarshaled.GroupName != reminder.GroupName {
		t.Errorf("GroupName mismatch: expected %q, got %q", reminder.GroupName, unmarshaled.GroupName)
	}
}
