package domain

// Command represents a user command
type Command interface {
	// Execute executes the command
	Execute(ctx Context) error

	// GetCommandName returns the name of the command
	GetCommandName() string
}

// Context holds execution context
type Context struct {
	UserID     string
	PlatformID string
	Platform   Platform
	AIService  AIService
}

// CommandExecutor executes commands based on user input
type CommandExecutor interface {
	// Execute executes user input and returns the result
	Execute(input string, ctx Context) (string, error)
}