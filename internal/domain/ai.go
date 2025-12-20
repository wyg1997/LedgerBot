package domain

import (
	"time"
)

type AIRequest struct {
	Model        string      `json:"model"`
	Messages     []AIMessage `json:"messages"`
	MaxTokens    int         `json:"max_tokens"`
	Temperature  float64     `json:"temperature"`
	Functions    []AIFunction `json:"functions,omitempty"`
	FunctionCall interface{}  `json:"function_call,omitempty"`
}

type AIMessage struct {
	Role         string       `json:"role"`
	Content      string       `json:"content"`
	FunctionCall *FunctionCall `json:"function_call,omitempty"`
}

type AIFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type AIResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int          `json:"index"`
	Message      AIMessage    `json:"message"`
	FinishReason string       `json:"finish_reason"`
	FunctionCall *FunctionCall `json:"function_call,omitempty"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// BillExtraction represents extracted bill information from AI
type BillExtraction struct {
	Description string   `json:"description"`
	Amount      float64  `json:"amount"`
	Type        string   `json:"type"`
	Category    string   `json:"category"`
	Date        string   `json:"date,omitempty"`
}

// RenameRequest represents a user rename request
type RenameRequest struct {
	Name string `json:"name"`
}

// CommandType represents the type of user command
type CommandType string

const (
	CommandTypeBill   CommandType = "bill"
	CommandTypeRename CommandType = "rename"
	CommandTypeStats  CommandType = "stats"
	CommandTypeHelp   CommandType = "help"
)

// AICommand represents a command determined by AI
 type AICommand struct {
	FunctionName string
	Arguments    map[string]interface{}
}

// AIService interface for AI integration
type AIService interface {
	// Execute processes user input via AI function calling
	Execute(input string, userName string, billService BillServiceInterface, renameService RenameServiceInterface) (string, error)
}

// BillServiceInterface defines functionality for handling bills in AI context
type BillServiceInterface interface {
	CreateBill(description string, amount float64, billType BillType, date *time.Time, category string) (*Bill, error)
}

// RenameServiceInterface defines functionality for renaming users in AI context
type RenameServiceInterface interface {
	Rename(name string) error
}