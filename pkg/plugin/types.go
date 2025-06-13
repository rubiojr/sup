package plugin

// MessageInfo contains information about the incoming message
type MessageInfo struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
	PushName  string `json:"push_name"`
	IsGroup   bool   `json:"is_group"`
}

// Input represents the input data passed to a plugin
type Input struct {
	Message string      `json:"message"`
	Sender  string      `json:"sender"`
	Info    MessageInfo `json:"info"`
}

// Output represents the response from a plugin
type Output struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Reply   string `json:"reply,omitempty"`
}

// HelpOutput contains help information for a plugin
type HelpOutput struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Usage       string   `json:"usage"`
	Examples    []string `json:"examples"`
	Category    string   `json:"category"`
}

// Plugin interface that plugin authors must implement
type Plugin interface {
	Name() string
	Topics() []string
	HandleMessage(input Input) Output
	GetHelp() HelpOutput
	GetRequiredEnvVars() []string
	Version() string
}

// Store interface for plugin storage operations
type Store interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte) error
}

// Success creates a successful output with a reply message
func Success(reply string) Output {
	return Output{
		Success: true,
		Reply:   reply,
	}
}

// Error creates an error output with an error message
func Error(message string) Output {
	return Output{
		Success: false,
		Error:   message,
	}
}

// NewHelpOutput creates a new HelpOutput with the given parameters
func NewHelpOutput(name, description, usage string, examples []string, category string) HelpOutput {
	return HelpOutput{
		Name:        name,
		Description: description,
		Usage:       usage,
		Examples:    examples,
		Category:    category,
	}
}
