package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/wyg1997/LedgerBot/config"
	"github.com/wyg1997/LedgerBot/internal/domain"
	"github.com/wyg1997/LedgerBot/pkg/logger"
)

// OpenAIService implements AIService with only function calling
type OpenAIService struct {
	config     *config.AIConfig
	httpClient *http.Client
	log        logger.Logger
}

// NewOpenAIService creates a new OpenAI service
func NewOpenAIService(cfg *config.AIConfig) domain.AIService {
	return &OpenAIService{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		log: logger.GetLogger(),
	}
}

// Execute processes user input via AI function calling
func (s *OpenAIService) Execute(input string, userName string, billService domain.BillServiceInterface, renameService domain.RenameServiceInterface, history []domain.AIMessage) (string, error) {
	functions := []domain.AIFunction{
		{
			Name:        "record_transaction",
			Description: "Record a financial transaction - expense or income",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"description": map[string]string{
						"type":        "string",
						"description": "Description of the transaction",
					},
					"amount": map[string]interface{}{
						"type":        "number",
						"description": "Amount of money (must be > 0)",
					},
					"type": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"expense", "income"},
						"description": "Type of transaction",
					},
					"category": map[string]string{
						"type":        "string",
						"description": "Category like food, transport, income",
					},
					"date": map[string]string{
						"type":        "string",
						"format":      "date",
						"description": "Date (YYYY-MM-DD), today if not specified",
					},
				},
				"required": []string{"description", "amount", "type", "category"},
			},
		},
		{
			Name:        "rename_user",
			Description: "Update user name based on their request",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]string{
						"type":        "string",
						"description": "New name for the user",
					},
				},
			},
		},
	}

	// Handle special case for unknown user
	systemPrompt := "You are a personal finance bot."
	if userName == "" {
		systemPrompt += " The user has not provided their name yet." +
			" If they introduce themselves as '我是XXX' or '叫我XXX' or similar, extract the name and call rename_user function." +
			" After setting the name, confirm it and proceed with normal service." +
			" Until they provide a name, you can still help with recording transactions."
	} else {
		systemPrompt += fmt.Sprintf(" Current user: %s."+
			" Always decide expense vs income based on description context.", userName)
	}

	systemPrompt += " Always decide expense vs income based on description context." +
		" '叫我XXX' or '我是XXX' means rename to XXX or extract name from user's introduction." +
		" Respond in Chinese."

	messages := []domain.AIMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}

	if len(history) > 0 {
		messages = append(messages, history...)
	} else {
		messages = append(messages, domain.AIMessage{
			Role:    "user",
			Content: input,
		})
	}

	req := domain.AIRequest{
		Model:        s.config.Model,
		Messages:     messages,
		Functions:    functions,
		FunctionCall: "auto",
	}

	resp, err := s.callAPI(req)
	if err != nil {
		s.log.Error("ai call: %v", err)
		return "抱歉，无法理解您的请求", err
	}

	choice := resp.Choices[0]

	// Direct response
	if choice.Message.FunctionCall == nil {
		return choice.Message.Content, nil
	}

	// Function call
	funcName := choice.Message.FunctionCall.Name
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(choice.Message.FunctionCall.Arguments), &args); err != nil {
		s.log.Error("parse args: %v", err)
		return "抱歉，参数解析失败", err
	}

	// Execute
	switch funcName {
	case "record_transaction":
		return s.handleRecordTransaction(args, billService.(*BillService))
	case "rename_user":
		return s.handleRenameUser(args, renameService.(*RenameService))
	}

	return "未知操作", fmt.Errorf("unknown function: %s", funcName)
}

func (s *OpenAIService) handleRecordTransaction(args map[string]interface{}, svc *BillService) (string, error) {
	description := getString(args, "description")
	amount := getFloat64(args, "amount")
	transType := getString(args, "type")
	category := getString(args, "category")

	if description == "" || amount <= 0 {
		return "请提供有效的交易信息", fmt.Errorf("invalid args")
	}

	var billDate *time.Time
	if ds := getString(args, "date"); ds != "" {
		if t, err := time.Parse("2006-01-02", ds); err == nil {
			billDate = &t
		}
	}

	bt := domain.BillTypeExpense
	if transType == "income" {
		bt = domain.BillTypeIncome
	}

	bill, err := svc.CreateBill(description, amount, bt, billDate, category)
	if err != nil {
		s.log.Error("create bill: %v", err)
		return "记账失败", err
	}

	sign := "-"
	if bill.Type == domain.BillTypeIncome {
		sign = "+"
	}

	return fmt.Sprintf("✅ 记账成功！\n📋 %s\n💰 %s¥%.2f\n🏷️ %s",
		bill.Description, sign, bill.Amount, bill.Category), nil
}

func (s *OpenAIService) handleRenameUser(args map[string]interface{}, svc *RenameService) (string, error) {
	name := getString(args, "name")
	if name == "" {
		return "名字不能为空", fmt.Errorf("empty name")
	}

	if err := svc.Rename(name); err != nil {
		s.log.Error("rename: %v", err)
		return "设置失败", err
	}

	return fmt.Sprintf("✅ 设置成功！从现在起，我将称呼您为：%s", name), nil
}

// BillService handles bill operations inside AI service
type BillService struct {
	billUseCase domain.BillUseCase
	userID      string
	userName    string
}

// NewBillService creates bill service for AI usage
func NewBillService(billUseCase domain.BillUseCase, userID string, userName string) domain.BillServiceInterface {
	return &BillService{billUseCase: billUseCase, userID: userID, userName: userName}
}

// CreateBill records new bill
func (s *BillService) CreateBill(description string, amount float64, billType domain.BillType, date *time.Time, category string) (*domain.Bill, error) {
	return s.billUseCase.CreateBill(s.userName, s.userID, "", description, amount, billType, date, &category)
}

// RenameService handles rename
type RenameService struct {
	userNameGet func() (string, error)
	userNameSet func(string) error
}

// NewRenameService creates rename service
func NewRenameService(setName func(string) error) domain.RenameServiceInterface {
	return &RenameService{
		userNameSet: setName,
	}
}

// Rename updates user name
func (s *RenameService) Rename(name string) error {
	return s.userNameSet(name)
}

// Remaining implementation helpers
func (s *OpenAIService) callAPI(req domain.AIRequest) (*domain.AIResponse, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("json marshal: %w", err)
	}

	httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/chat/completions", s.config.BaseURL), bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.config.APIKey))

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error %d: %s", resp.StatusCode, string(body))
	}

	result := &domain.AIResponse{}
	json.Unmarshal(body, result)
	return result, nil
}

func getString(m map[string]interface{}, key string) string {
	v, ok := m[key].(string)
	if !ok {
		return ""
	}
	return v
}

func getFloat64(m map[string]interface{}, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}
