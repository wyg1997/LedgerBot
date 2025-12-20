package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"github.com/wyg1997/LedgerBot/config"
	"github.com/wyg1997/LedgerBot/internal/domain"
	"github.com/wyg1997/LedgerBot/pkg/logger"
)

// OpenAIService implements AIService with only function calling
type OpenAIService struct {
	config *config.AIConfig
	client *openai.Client
	log    logger.Logger
}

// NewOpenAIService creates a new OpenAI service
func NewOpenAIService(cfg *config.AIConfig) domain.AIService {
	// 使用 go-openai 官方/社区 SDK，优先支持自定义 BaseURL
	openaiCfg := openai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		// go-openai 期望的是包含 /v1 的完整前缀
		baseURL := cfg.BaseURL
		if baseURL[len(baseURL)-1] == '/' {
			baseURL = baseURL[:len(baseURL)-1]
		}
		openaiCfg.BaseURL = fmt.Sprintf("%s/v1", baseURL)
	}

	return &OpenAIService{
		config: cfg,
		client: openai.NewClientWithConfig(openaiCfg),
		log:    logger.GetLogger(),
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
		// 未知用户名时：只能识别改名意图，其它内容统一要求用户先提供称呼
		systemPrompt += " The user has not provided their name yet." +
			" If they introduce themselves as '我是XXX' or '叫我XXX' or similar, you MUST extract the name and call rename_user function." +
			" For any other request (including recording transactions, statistics, or normal chat), you MUST politely ask the user to first tell you how to address them, and DO NOT perform any other operation until a name is set."
	} else {
		systemPrompt += fmt.Sprintf(" Current user: %s.", userName)
	}

	systemPrompt += " Always decide expense vs income based on description context when recording transactions." +
		" '叫我XXX' or '我是XXX' means rename to XXX or extract name from the user's introduction." +
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

	// 构造 ChatCompletion 请求
	openaiMessages := make([]openai.ChatCompletionMessage, 0, len(messages))
	for _, m := range messages {
		openaiMessages = append(openaiMessages, openai.ChatCompletionMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	openaiFunctions := make([]openai.FunctionDefinition, 0, len(functions))
	for _, f := range functions {
		paramsJSON, err := json.Marshal(f.Parameters)
		if err != nil {
			s.log.Error("marshal function params: %v", err)
			continue
		}
		openaiFunctions = append(openaiFunctions, openai.FunctionDefinition{
			Name:        f.Name,
			Description: f.Description,
			Parameters:  paramsJSON,
		})
	}

	req := openai.ChatCompletionRequest{
		Model:        s.config.Model,
		Messages:     openaiMessages,
		Functions:    openaiFunctions,
		FunctionCall: "auto",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := s.client.CreateChatCompletion(ctx, req)
	if err != nil {
		s.log.Error("ai call: %v", err)
		return "抱歉，无法理解您的请求", err
	}

	if len(resp.Choices) == 0 {
		return "抱歉，没有获得有效的AI响应", fmt.Errorf("empty choices from OpenAI")
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

	// 如果尚未识别出用户名，但 AI 试图调用的并不是改名函数，则拦截并提示用户先提供称呼
	if userName == "" && funcName != "rename_user" {
		s.log.Debug("block function %s for unknown user, ask for name first", funcName)
		return "我还不知道您是谁？请告诉我您的称呼。\n您可以直接说：我是张三", nil
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
