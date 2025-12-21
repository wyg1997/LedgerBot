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
	// 使用 go-openai Config，以便支持自定义 BaseURL
	openaiCfg := openai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		baseURL := cfg.BaseURL
		// 去掉末尾的斜杠，避免重复 //
		if baseURL[len(baseURL)-1] == '/' {
			baseURL = baseURL[:len(baseURL)-1]
		}
		// go-openai 期望的是包含 /v1 的完整前缀
		openaiCfg.BaseURL = fmt.Sprintf("%s/v1", baseURL)
	}

	return &OpenAIService{
		config: cfg,
		client: openai.NewClientWithConfig(openaiCfg),
		log:    logger.GetLogger(),
	}
}

// Execute processes user input via AI tool-calling using go-openai Tools API
func (s *OpenAIService) Execute(input string, userName string, billService domain.BillServiceInterface, renameService domain.RenameServiceInterface, history []domain.AIMessage) (string, error) {
	// 1. System prompt
	systemPrompt := "You are a personal finance bot."
	if userName == "" {
		systemPrompt += " The user has not provided their name yet." +
			" If they introduce themselves as '我是XXX' or '叫我XXX' or similar, you MUST extract the name and call rename_user function." +
			" For any other request (including recording transactions, statistics, or normal chat), you MUST politely ask the user to first tell you how to address them, and DO NOT perform any other operation until a name is set."
	} else {
		systemPrompt += fmt.Sprintf(" Current user: %s.", userName)
	}
	systemPrompt += " Always decide expense vs income based on description context when recording transactions." +
		" When recording transactions, the date is automatically set to the current date by the server, so you should NOT ask for or use date information from the user." +
		" CRITICAL RULE FOR CATEGORY SELECTION: When calling record_transaction, you MUST automatically select a category from the enum list (餐饮, 交通, 购物, 娱乐, 医疗, 教育, 住房, 水电费, 通讯, 服装, 收入, 其它) WITHOUT asking the user. NEVER ask questions like '这是什么分类？', '请选择分类', '这是什么类型的支出？' or any similar questions about category. Just analyze the transaction description and immediately choose the most appropriate category. If you're unsure, use '其它'. This is mandatory - you must always provide a category value, never leave it empty or ask the user to choose." +
		" MULTIPLE TRANSACTIONS: If the user mentions multiple transactions in a single message (e.g., '午饭30元，打车45元' or '今天花了30块吃饭，45块打车'), you MUST call record_transaction MULTIPLE TIMES - once for each transaction. You can make multiple tool calls in a single response. Each transaction should be recorded separately with its own record_transaction call. Do NOT combine multiple transactions into a single record_transaction call." +
		" When calling record_transaction, you should provide the original_message parameter with the most relevant user message from the conversation that best represents what the user said about this transaction." +
		" For thread conversations, extract the most appropriate user message from the conversation history that led to this transaction." +
		" '叫我XXX' or '我是XXX' means rename to XXX or extract name from the user's introduction." +
		" Respond in Chinese."

	// 2. Build messages (system + history or current input)
	msgs := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
	}

	if len(history) > 0 {
		for _, m := range history {
			role := openai.ChatMessageRoleUser
			if m.Role == "system" {
				role = openai.ChatMessageRoleSystem
			} else if m.Role == "assistant" {
				role = openai.ChatMessageRoleAssistant
			}
			msgs = append(msgs, openai.ChatCompletionMessage{
				Role:    role,
				Content: m.Content,
			})
		}
	} else {
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: input,
		})
	}

	// 3. Define tools: record_transaction & rename_user
	tools := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "record_transaction",
				Description: "Record a financial transaction - expense or income. You MUST automatically select the category from the enum list without asking the user. Never ask for category confirmation - just choose the most appropriate one based on the transaction description.",
				Parameters: mustMarshalJSON(map[string]interface{}{
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
						"category": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"餐饮", "交通", "购物", "娱乐", "医疗", "教育", "住房", "水电费", "通讯", "服装", "收入", "其它"},
							"description": "Transaction category. CRITICAL: You MUST automatically select a category from this enum list WITHOUT asking the user. NEVER ask '这是什么分类？' or '请选择分类' or any similar questions. Just analyze the transaction description and choose the most appropriate category immediately. Available categories: 餐饮(food/dining), 交通(transportation), 购物(shopping), 娱乐(entertainment), 医疗(medical), 教育(education), 住房(housing), 水电费(utilities), 通讯(communication), 服装(clothing), 收入(income), 其它(other). If unsure, use '其它'. This is a required parameter - you must provide a value, never ask the user to choose.",
						},
						"original_message": map[string]string{
							"type":        "string",
							"description": "The original user message that led to this transaction. For thread conversations, extract the most relevant user message from the conversation history that best represents what the user said about this transaction.",
						},
					},
					"required": []string{"description", "amount", "type", "category"},
				}),
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "rename_user",
				Description: "Update user name based on their request",
				Parameters: mustMarshalJSON(map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]string{
							"type":        "string",
							"description": "New name for the user",
						},
					},
					"required": []string{"name"},
				}),
			},
		},
	}

	// 4. Build request
	req := openai.ChatCompletionRequest{
		Model:    s.config.Model,
		Messages: msgs,
		Tools:    tools,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 5. Call CreateChatCompletion
	resp, err := s.client.CreateChatCompletion(ctx, req)
	if err != nil {
		s.log.Error("ai call: %v", err)
		return "抱歉，无法理解您的请求", err
	}
	if len(resp.Choices) == 0 {
		return "抱歉，没有获得有效的AI响应", fmt.Errorf("empty choices")
	}

	choice := resp.Choices[0]
	msg := choice.Message

	// Debug: Print full AI response
	s.log.Debug("AI response received: role=%s, content=%s, toolCallsCount=%d", msg.Role, msg.Content, len(msg.ToolCalls))
	if len(msg.ToolCalls) > 0 {
		for i, tc := range msg.ToolCalls {
			s.log.Debug("ToolCall[%d]: id=%s, type=%s, function.name=%s, function.arguments=%s",
				i, tc.ID, tc.Type, tc.Function.Name, tc.Function.Arguments)
		}
	}

	// 6. No tool call: return assistant reply directly
	if len(msg.ToolCalls) == 0 {
		return msg.Content, nil
	}

	// 7. Handle tool calls locally (record_transaction / rename_user)
	// Support multiple toolcalls - process all and return combined result
	var results []string
	var hasError bool

	for _, tc := range msg.ToolCalls {
		fn := tc.Function
		if fn.Name == "" {
			continue
		}

		name := fn.Name
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(fn.Arguments), &args); err != nil {
			s.log.Error("parse tool args: %v", err)
			results = append(results, fmt.Sprintf("❌ %s: 参数解析失败", name))
			hasError = true
			continue
		}

		s.log.Info("AI toolcall triggered: tool=%s, user=%s, args=%+v", name, userName, args)

		// 未知用户时，只允许 rename_user
		if userName == "" && name != "rename_user" {
			s.log.Info("Blocking tool %s for unknown user, asking for name first", name)
			return "我还不知道您是谁？请告诉我您的称呼。\n您可以直接说：我是张三", nil
		}

		var result string
		var err error

		switch name {
		case "record_transaction":
			result, err = s.handleRecordTransaction(args, billService.(*BillService))
		case "rename_user":
			result, err = s.handleRenameUser(args, renameService.(*RenameService))
		default:
			s.log.Error("Unknown tool call: %s", name)
			results = append(results, fmt.Sprintf("❌ 未知操作: %s", name))
			hasError = true
			continue
		}

		if err != nil {
			s.log.Error("Tool call %s failed: %v", name, err)
			results = append(results, fmt.Sprintf("❌ %s 执行失败: %v", name, err))
			hasError = true
		} else {
			results = append(results, result)
		}
	}

	// Return combined results
	if len(results) == 0 {
		return "未知操作", fmt.Errorf("no valid tool calls")
	}

	// If all succeeded, join with double newlines for better separation; if any failed, indicate error
	response := ""
	if hasError {
		response = "部分操作完成：\n" + fmt.Sprintf("%s\n", results[0])
		for i := 1; i < len(results); i++ {
			response += results[i] + "\n"
		}
	} else {
		// Multiple successful transactions: separate with double newlines
		response = results[0]
		for i := 1; i < len(results); i++ {
			response += "\n\n" + results[i]
		}
	}

	return response, nil
}

// mustMarshalJSON is a small helper to build json.RawMessage
func mustMarshalJSON(v interface{}) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}

func (s *OpenAIService) handleRecordTransaction(args map[string]interface{}, svc *BillService) (string, error) {
	description := getString(args, "description")
	amount := getFloat64(args, "amount")
	transType := getString(args, "type")
	category := getString(args, "category")
	originalMsg := getString(args, "original_message")

	if description == "" || amount <= 0 {
		s.log.Error("Invalid transaction args: description=%s, amount=%.2f", description, amount)
		return "请提供有效的交易信息", fmt.Errorf("invalid args")
	}

	// 日期由服务器自动使用当前时间，不接收 AI 传入的日期参数
	bt := domain.BillTypeExpense
	if transType == "income" {
		bt = domain.BillTypeIncome
	}

	bill, err := svc.CreateBill(description, amount, bt, nil, category, originalMsg)
	if err != nil {
		s.log.Error("Failed to create bill: %v", err)
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
		s.log.Error("Empty name provided for rename_user")
		return "名字不能为空", fmt.Errorf("empty name")
	}

	if err := svc.Rename(name); err != nil {
		s.log.Error("Failed to rename user: %v", err)
		return "设置失败", err
	}

	return fmt.Sprintf("✅ 设置成功！从现在起，我将称呼您为：%s", name), nil
}

// BillService handles bill operations inside AI service
type BillService struct {
	billUseCase domain.BillUseCase
	userID      string
	userName    string
	originalMsg string
}

// NewBillService creates bill service for AI usage
func NewBillService(billUseCase domain.BillUseCase, userID string, userName string, originalMsg string) domain.BillServiceInterface {
	return &BillService{
		billUseCase: billUseCase,
		userID:      userID,
		userName:    userName,
		originalMsg: originalMsg,
	}
}

// CreateBill records new bill
func (s *BillService) CreateBill(description string, amount float64, billType domain.BillType, date *time.Time, category string, originalMsg string) (*domain.Bill, error) {
	// Use originalMsg from AI toolcall parameter, fallback to stored originalMsg if not provided
	if originalMsg == "" {
		originalMsg = s.originalMsg
	}
	return s.billUseCase.CreateBill(s.userName, s.userID, originalMsg, description, amount, billType, date, &category)
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
