package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"github.com/wyg1997/LedgerBot/config"
	"github.com/wyg1997/LedgerBot/internal/domain"
	"github.com/wyg1997/LedgerBot/internal/infrastructure/repository"
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
	// Get current year dynamically
	currentYear := time.Now().Year()
	
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
		" UPDATE TRANSACTIONS: If the user wants to update an existing transaction, use the update_transaction tool. The user will provide the record_id (from the original transaction response, shown as 🆔). You can update one or more fields (description, amount, type, category). If the user mentions multiple updates in a single message, you MUST call update_transaction MULTIPLE TIMES - once for each record that needs to be updated. Only include fields that the user wants to change - do not include unchanged fields. NOTE: The original_message field will be automatically updated with the user's current update instruction - you do NOT need to include it in the tool call." +
		" DELETE TRANSACTIONS: If the user wants to delete an existing transaction, use the delete_transaction tool. The user will provide the record_id (from the original transaction response, shown as 🆔). If the user mentions multiple deletions in a single message, you MUST call delete_transaction MULTIPLE TIMES - once for each record that needs to be deleted." +
		fmt.Sprintf(" QUERY TRANSACTIONS: If the user wants to query or view their transaction history, use the query_transaction tool. Supported time ranges: 'today', 'yesterday', 'this_week', 'last_week', 'this_month', 'last_month', 'last_7_days', 'last_30_days', or 'custom' for specific date ranges. IMPORTANT: When user mentions dates without year (e.g., '12月1日', '1月15日', '12月1号到12月10号'), you MUST infer the current year (%d) and use 'custom' type with full date format 'YYYY-MM-DD hh:mm:ss'. If only date is provided without time, start_time defaults to 00:00:00 and end_time defaults to 23:59:59. The user may also request a specific number of top transactions (e.g., 'top 10', '前10条', '显示前20条'), which you should set in the top_n parameter (default is 5).", currentYear) +
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
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "update_transaction",
				Description: "Update an existing financial transaction record. Use this when the user wants to modify a previously recorded transaction. You need the record_id from the original transaction record.",
				Parameters: mustMarshalJSON(map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"record_id": map[string]string{
							"type":        "string",
							"description": "The record_id of the transaction to update (from the original record response)",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "Updated description of the transaction (optional, only include if user wants to change it)",
						},
						"amount": map[string]interface{}{
							"type":        "number",
							"description": "Updated amount of money (optional, only include if user wants to change it, must be > 0)",
						},
						"type": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"expense", "income"},
							"description": "Updated type of transaction (optional, only include if user wants to change it)",
						},
						"category": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"餐饮", "交通", "购物", "娱乐", "医疗", "教育", "住房", "水电费", "通讯", "服装", "收入", "其它"},
							"description": "Updated transaction category (optional, only include if user wants to change it). CRITICAL: You MUST automatically select a category from this enum list WITHOUT asking the user if category needs to be updated.",
						},
						"original_message": map[string]interface{}{
							"type":        "string",
							"description": "This field will be automatically updated with the user's current update instruction/command. You do NOT need to provide this parameter - it is handled automatically by the system. Only include if you have a specific reason to override the automatic value.",
						},
					},
					"required": []string{"record_id"},
				}),
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "delete_transaction",
				Description: "Delete an existing financial transaction record. Use this when the user wants to remove a previously recorded transaction. You need the record_id from the original transaction record.",
				Parameters: mustMarshalJSON(map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"record_id": map[string]string{
							"type":        "string",
							"description": "The record_id of the transaction to delete (from the original record response, shown as 🆔)",
						},
					},
					"required": []string{"record_id"},
				}),
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "query_transactions",
				Description: "Query financial transactions within a specified time range. Use this when the user wants to view their transaction history, check spending, or see financial summaries.",
				Parameters: mustMarshalJSON(map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"time_range_type": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"today", "yesterday", "this_week", "last_week", "this_month", "last_month", "last_7_days", "last_30_days", "custom"},
							"description": fmt.Sprintf("Time range type. Use predefined ranges (today, yesterday, this_week, last_week, this_month, last_month, last_7_days, last_30_days) or 'custom' for specific date ranges. IMPORTANT: When user mentions dates without year (e.g., '12月1日', '1月15日'), you MUST infer the current year (%d) and use 'custom' type with full date format.", currentYear),
						},
						"start_time": map[string]string{
							"type":        "string",
							"description": fmt.Sprintf("Start time in format 'YYYY-MM-DD hh:mm:ss' (required only if time_range_type is 'custom'). If only date is provided without time, it will default to 00:00:00. MUST include year (e.g., '%d-12-19 00:00:00').", currentYear),
						},
						"end_time": map[string]string{
							"type":        "string",
							"description": fmt.Sprintf("End time in format 'YYYY-MM-DD hh:mm:ss' (required only if time_range_type is 'custom'). If only date is provided without time, it will default to 23:59:59. MUST include year (e.g., '%d-12-19 23:59:59').", currentYear),
						},
						"top_n": map[string]interface{}{
							"type":        "integer",
							"description": "Number of top transactions to return (sorted by amount descending). Default is 5. User may request a different number (e.g., 'top 10', '前10条').",
							"default":     5,
						},
					},
					"required": []string{"time_range_type"},
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
		case "update_transaction":
			// Pass current input so we can use it as original_message for updates
			result, err = s.handleUpdateTransaction(args, billService.(*BillService), input)
		case "delete_transaction":
			result, err = s.handleDeleteTransaction(args, billService.(*BillService))
		case "query_transactions":
			result, err = s.handleQueryTransactions(args, billService.(*BillService))
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

	// Include record_id in response for future updates
	response := fmt.Sprintf("✅ 记账成功！\n📋 %s\n💰 %s¥%.2f\n🏷️ %s",
		bill.Description, sign, bill.Amount, bill.Category)
	
	if bill.RecordID != "" {
		response += fmt.Sprintf("\n🆔 %s", bill.RecordID)
	}

	return response, nil
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

func (s *OpenAIService) handleUpdateTransaction(args map[string]interface{}, svc *BillService, currentInput string) (string, error) {
	recordID := getString(args, "record_id")
	if recordID == "" {
		s.log.Error("Missing record_id in update_transaction args")
		return "请提供记录ID", fmt.Errorf("record_id is required")
	}

	// Extract optional update fields
	var description *string
	var amount *float64
	var billType *domain.BillType
	var category *string
	var originalMsg *string

	if desc := getString(args, "description"); desc != "" {
		description = &desc
	}
	if amt := getFloat64(args, "amount"); amt > 0 {
		amount = &amt
	}
	if transType := getString(args, "type"); transType != "" {
		bt := domain.BillTypeExpense
		if transType == "income" {
			bt = domain.BillTypeIncome
		}
		billType = &bt
	}
	if cat := getString(args, "category"); cat != "" {
		category = &cat
	}
	
	// Get the original bill to retrieve the existing original_message
	// We need to combine the original message with the current update instruction
	originalBill, err := svc.billUseCase.GetBill(recordID)
	if err != nil {
		s.log.Error("Failed to get original bill for update: %v", err)
		// If we can't get the original bill, just use current input as original_message
		if currentInput != "" {
			originalMsg = &currentInput
		}
	} else {
		// Combine original message with current update instruction
		combinedMsg := originalBill.OriginalMsg
		if combinedMsg != "" && currentInput != "" {
			combinedMsg = combinedMsg + " | " + currentInput
		} else if currentInput != "" {
			combinedMsg = currentInput
		} else if combinedMsg == "" {
			// Fallback to AI-provided original_message if both are empty
			if origMsg := getString(args, "original_message"); origMsg != "" {
				combinedMsg = origMsg
			}
		}
		if combinedMsg != "" {
			originalMsg = &combinedMsg
		}
	}

	// Check if at least one field is being updated
	if description == nil && amount == nil && billType == nil && category == nil && originalMsg == nil {
		return "请提供至少一个要更新的字段", fmt.Errorf("no fields to update")
	}

	bill, err := svc.UpdateBill(recordID, description, amount, billType, category, originalMsg)
	if err != nil {
		s.log.Error("Failed to update bill: %v", err)
		return "更新失败", err
	}

	sign := "-"
	if bill.Type == domain.BillTypeIncome {
		sign = "+"
	}

	response := fmt.Sprintf("✅ 更新成功！\n📋 %s\n💰 %s¥%.2f\n🏷️ %s",
		bill.Description, sign, bill.Amount, bill.Category)
	
	if bill.RecordID != "" {
		response += fmt.Sprintf("\n🆔 %s", bill.RecordID)
	}

	return response, nil
}

func (s *OpenAIService) handleDeleteTransaction(args map[string]interface{}, svc *BillService) (string, error) {
	recordID := getString(args, "record_id")
	if recordID == "" {
		s.log.Error("Missing record_id in delete_transaction args")
		return "请提供记录ID", fmt.Errorf("record_id is required")
	}

	err := svc.DeleteBill(recordID)
	if err != nil {
		s.log.Error("Failed to delete bill: %v", err)
		return "删除失败", err
	}

	return fmt.Sprintf("✅ 删除成功！\n🆔 %s", recordID), nil
}

func (s *OpenAIService) handleQueryTransactions(args map[string]interface{}, svc *BillService) (string, error) {
	timeRangeTypeStr := getString(args, "time_range_type")
	if timeRangeTypeStr == "" {
		s.log.Error("Missing time_range_type in query_transactions args")
		return "请提供时间范围类型", fmt.Errorf("time_range_type is required")
	}

	// Parse time range
	var startTime, endTime time.Time
	var err error

	timeRangeType := repository.TimeRangeType(timeRangeTypeStr)
	if timeRangeType == repository.TimeRangeCustom {
		startTimeStr := getString(args, "start_time")
		endTimeStr := getString(args, "end_time")
		if startTimeStr == "" || endTimeStr == "" {
			s.log.Error("Missing start_time or end_time for custom time range")
			return "自定义时间范围需要提供开始时间和结束时间", fmt.Errorf("start_time and end_time are required for custom time range")
		}
		startTime, endTime, err = repository.ParseTimeRange(timeRangeType, startTimeStr, endTimeStr)
	} else {
		startTime, endTime, err = repository.ParseTimeRange(timeRangeType, "", "")
	}

	if err != nil {
		s.log.Error("Failed to parse time range: %v", err)
		return "时间范围解析失败", err
	}

	// Get top_n (default 5)
	topN := 5
	if topNVal, ok := args["top_n"]; ok {
		if topNFloat, ok := topNVal.(float64); ok {
			topN = int(topNFloat)
		}
	}

	// Query transactions
	bills, totalIncome, totalExpense, err := svc.QueryTransactions(startTime, endTime, topN)
	if err != nil {
		s.log.Error("Failed to query transactions: %v", err)
		return "查询失败", err
	}

	// Format response
	netAmount := totalIncome - totalExpense
	response := fmt.Sprintf("📊 查询结果（%s 至 %s）\n\n", 
		startTime.Format("2006-01-02"), endTime.Format("2006-01-02"))
	response += fmt.Sprintf("💰 总收入: ¥%.2f\n", totalIncome)
	response += fmt.Sprintf("💸 总支出: ¥%.2f\n", totalExpense)
	response += fmt.Sprintf("📈 净收支: ¥%.2f\n\n", netAmount)

	if len(bills) > 0 {
		response += fmt.Sprintf("🔝 Top %d 交易记录:\n", len(bills))
		for i, bill := range bills {
			sign := "-"
			if bill.Type == domain.BillTypeIncome {
				sign = "+"
			}
			response += fmt.Sprintf("%d. %s %s¥%.2f [%s]\n", 
				i+1, bill.Description, sign, bill.Amount, bill.Category)
			if bill.RecordID != "" {
				response += fmt.Sprintf("   🆔 %s\n", bill.RecordID)
			}
		}
	} else {
		response += "📝 暂无交易记录\n"
	}

	return response, nil
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

// UpdateBill updates an existing bill by record_id
// Directly updates without querying - only updates fields that are provided
func (s *BillService) UpdateBill(recordID string, description *string, amount *float64, billType *domain.BillType, category *string, originalMsg *string) (*domain.Bill, error) {
	// Build updates map with only the fields that are provided
	updates := make(map[string]interface{})
	if description != nil {
		updates["description"] = *description
	}
	if amount != nil {
		updates["amount"] = *amount
	}
	if billType != nil {
		updates["type"] = *billType
	}
	if category != nil {
		updates["category"] = *category
	}
	if originalMsg != nil {
		updates["original_message"] = *originalMsg
	}
	
	// Use case UpdateBill will detect record_id (starts with "rec") and update directly without querying
	updatedBill, err := s.billUseCase.UpdateBill(recordID, updates)
	if err != nil {
		return nil, err
	}
	
	// Ensure record_id is set in the returned bill
	updatedBill.RecordID = recordID
	
	return updatedBill, nil
}

// DeleteBill deletes an existing bill by record_id
func (s *BillService) DeleteBill(recordID string) error {
	return s.billUseCase.DeleteBill(recordID)
}

// QueryTransactions queries transactions within a time range
func (s *BillService) QueryTransactions(startTime, endTime time.Time, topN int) ([]*domain.Bill, float64, float64, error) {
	return s.billUseCase.QueryTransactions(s.userName, startTime, endTime, topN)
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
