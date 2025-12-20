package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/wyg1997/LedgerBot/config"
	"github.com/wyg1997/LedgerBot/internal/domain"
	"github.com/wyg1997/LedgerBot/internal/infrastructure/ai"
	"github.com/wyg1997/LedgerBot/internal/infrastructure/platform/feishu"
	"github.com/wyg1997/LedgerBot/pkg/logger"
)

// FeishuHandlerAITools processes requests using AI tool calling
type FeishuHandlerAITools struct {
	config          *config.FeishuConfig
	feishuService   *feishu.FeishuService
	billUseCase     domain.BillUseCase
	aiservice       domain.AIService
	userMappingRepo domain.UserMappingRepository
	logger          logger.Logger
}

// NewFeishuHandlerAITools creates handler
func NewFeishuHandlerAITools(
	config *config.FeishuConfig,
	feishuService *feishu.FeishuService,
	billUseCase domain.BillUseCase,
	aiservice domain.AIService,
	userMappingRepo domain.UserMappingRepository,
) *FeishuHandlerAITools {
	return &FeishuHandlerAITools{
		config:          config,
		feishuService:   feishuService,
		billUseCase:     billUseCase,
		aiservice:       aiservice,
		userMappingRepo: userMappingRepo,
		logger:          logger.GetLogger(),
	}
}

// ExecuteFunc creates the service wrappers for AI execution
func (h *FeishuHandlerAITools) ExecuteFunc(openID string, userName string, renameFunc func(string) error) func(string, string, domain.BillUseCase, func(string) error, []domain.AIMessage) (string, error) {
	return func(input string, name string, billUseCase domain.BillUseCase, renameFunc func(string) error, history []domain.AIMessage) (string, error) {
		// Create bill service wrapper - use a default user ID since we don't track users anymore
		billService := ai.NewBillService(billUseCase, openID, name)
		// Create rename service wrapper
		renameService := ai.NewRenameService(renameFunc)

		// Call the proper Execute method
		return h.aiservice.Execute(input, name, billService, renameService, history)
	}
}

// Webhook processes Feishu webhook
func (h *FeishuHandlerAITools) Webhook(w http.ResponseWriter, r *http.Request) {
	// Log the incoming request
	h.logger.Debug("=== Received Feishu Webhook Request ===")
	h.logger.Debug("Method: %s", r.Method)
	h.logger.Debug("URL: %s", r.URL.String())
	h.logger.Debug("Headers: %v", r.Header)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("read body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		h.logger.Error("json unmarshal: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Log the received payload
	h.logger.Debug("Payload: %s", string(body))
	if challenge, ok := payload["challenge"]; ok {
		h.logger.Debug("Challenge received: %v", challenge)
	}

	// Handle challenge
	if challenge := payload["challenge"]; challenge != nil {
		json.NewEncoder(w).Encode(map[string]string{"challenge": fmt.Sprintf("%v", challenge)})
		return
	}

	// 检查是否是新的IM消息格式 (event_type 在 header 中)
	header := getMap(payload, "header")
	if header != nil {
		eventType := getString(header, "event_type")
		if eventType == "im.message.receive_v1" {
			h.logger.Debug("检测到新的IM消息格式，调用处理函数")
			h.handleIMMessage(w, payload)
			return
		}
	}

	// 如果没有header.event_type = im.message.receive_v1，则直接返回ok
	h.logger.Debug("Unknown message format, returning ok")
	w.Write([]byte("ok"))
}

func (h *FeishuHandlerAITools) processMessage(openID, text, messageID string, history []domain.AIMessage) {
	h.logger.Info("Processing from %s: %s", openID, text)

	userName, err := h.ensureUser(openID, messageID)
	if err != nil {
		return
	}
	h.logger.Info("用户名: %s", userName)

	// Rename function - simplifies to just updating stored name
	renameFunc := func(name string) error {
		return h.userMappingRepo.SetUserName(openID, name)
	}

	// Execute via tool service
	toolService := h.ExecuteFunc(openID, userName, renameFunc)
	response, err := toolService(text, userName, h.billUseCase, renameFunc, history)
	if err != nil {
		h.logger.Error("AI execution: %v", err)
		// Use ReplyMessage with UUID for error response
		errMsg := fmt.Sprintf("AI处理失败：%v", err)
		_ = h.feishuService.ReplyMessage(messageID, errMsg, uuid.New().String())
		return
	}

	// Use ReplyMessage with UUID for successful response
	_ = h.feishuService.ReplyMessage(messageID, response, uuid.New().String())
}

func (h *FeishuHandlerAITools) ensureUser(openID, messageID string) (string, error) {
	// Try to get user name from mapping
	userName, err := h.userMappingRepo.GetUserName(openID)
	if err == nil {
		h.logger.Debug("获取用户映射: %s -> %s", openID, userName)
		return userName, nil
	}

	// User not found, ask them to provide their name
	replyMsg := "我还不知道您是谁？请告诉我您的称呼。\n您可以直接说：我是张三"
	_ = h.feishuService.ReplyMessage(messageID, replyMsg, uuid.New().String())
	return "", fmt.Errorf("unknown user")
}

func getString(m map[string]interface{}, key string) string {
	v, ok := m[key].(string)
	if !ok {
		return ""
	}
	return v
}

func getBool(m map[string]interface{}, key string) bool {
	v, ok := m[key].(bool)
	return ok && v
}

func getMap(m map[string]interface{}, key string) map[string]interface{} {
	v, ok := m[key].(map[string]interface{})
	if !ok {
		return nil
	}
	return v
}

// getObjectKeys returns a slice of string keys in the map
func getObjectKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// checkAndStripMention 判断当前消息是否@Bot并去掉文本中的@占位
func (h *FeishuHandlerAITools) checkAndStripMention(text string, message map[string]interface{}, botName string) (bool, string) {
	mentions := message["mentions"]
	if mentions == nil {
		return false, text
	}
	mentionList, ok := mentions.([]interface{})
	if !ok || len(mentionList) == 0 {
		return false, text
	}

	for _, mention := range mentionList {
		mentionMap, ok := mention.(map[string]interface{})
		if !ok {
			continue
		}
		name := getString(mentionMap, "name")
		mentionKey := getString(mentionMap, "key")

		if name == botName {
			if mentionKey != "" && strings.Contains(text, mentionKey) {
				text = strings.TrimSpace(strings.Replace(text, mentionKey, "", 1))
			}
			return true, text
		}
	}

	return false, text
}

// firstMessageMentionsBot 判断线程第一条消息是否@了机器人
func (h *FeishuHandlerAITools) firstMessageMentionsBot(messages []*larkim.Message, botName string) bool {
	if len(messages) == 0 {
		return false
	}

	return h.messageMentionsBot(messages[0], botName)
}

// messageMentionsBot 判断单条消息的mentions中是否包含Bot
func (h *FeishuHandlerAITools) messageMentionsBot(msg *larkim.Message, botName string) bool {
	if msg == nil || msg.Mentions == nil {
		return false
	}

	for _, mention := range msg.Mentions {
		if mention == nil || mention.Name == nil {
			continue
		}
		if *mention.Name == botName {
			return true
		}
	}

	return false
}

// buildAIHistoryFromThread 构建AI上下文，映射sender_type到角色
func (h *FeishuHandlerAITools) buildAIHistoryFromThread(messages []*larkim.Message, botName string) []domain.AIMessage {
	history := make([]domain.AIMessage, 0, len(messages))

	for _, msg := range messages {
		if msg == nil {
			continue
		}

		if msg.Deleted != nil && *msg.Deleted {
			continue
		}

		body := msg.Body
		if body == nil || body.Content == nil {
			continue
		}

		var contentObj map[string]interface{}
		if err := json.Unmarshal([]byte(*body.Content), &contentObj); err != nil {
			continue
		}

		text := getString(contentObj, "text")
		if text == "" {
			continue
		}

		// 去掉@Bot的key，避免AI误判
		if h.messageMentionsBot(msg, botName) && msg.Mentions != nil {
			for _, mention := range msg.Mentions {
				if mention == nil || mention.Key == nil {
					continue
				}
				if strings.Contains(text, *mention.Key) {
					text = strings.TrimSpace(strings.Replace(text, *mention.Key, "", 1))
					break
				}
			}
		}

		role := "user"
		if msg.Sender != nil && msg.Sender.SenderType != nil && *msg.Sender.SenderType == "app" {
			role = "assistant"
		}

		history = append(history, domain.AIMessage{
			Role:    role,
			Content: text,
		})
	}

	return history
}

// handleIMMessage handles the new IM message format (im.message.receive_v1)
func (h *FeishuHandlerAITools) handleIMMessage(w http.ResponseWriter, payload map[string]interface{}) {
	h.logger.Debug("=== Processing new IM message format ===")

	// Extract header info
	header := getMap(payload, "header")
	if header == nil {
		h.logger.Debug("No header found in payload")
		w.Write([]byte("ok"))
		return
	}

	eventType := getString(header, "event_type")
	eventID := getString(header, "event_id")
	h.logger.Debug("Header info - event_type: %s, event_id: %s", eventType, eventID)

	// Extract event info
	event := getMap(payload, "event")
	if event == nil {
		h.logger.Debug("No event found in payload, keys: %v", getObjectKeys(payload))
		w.Write([]byte("ok"))
		return
	}

	// Extract message details
	message := getMap(event, "message")
	if message == nil {
		h.logger.Debug("No message found in event, event keys: %v", getObjectKeys(event))
		w.Write([]byte("ok"))
		return
	}

	// Log message basics
	chatID := getString(message, "chat_id")
	chatType := getString(message, "chat_type")
	messageType := getString(message, "message_type")
	h.logger.Debug("Message info - chat_id: %s, chat_type: %s, message_type: %s", chatID, chatType, messageType)

	// Extract sender info
	sender := getMap(event, "sender")
	if sender == nil {
		h.logger.Debug("No sender found in message")
		w.Write([]byte("ok"))
		return
	}

	senderID := getMap(sender, "sender_id")
	if senderID == nil {
		h.logger.Debug("No sender_id found in sender")
		w.Write([]byte("ok"))
		return
	}

	// Get sender details
	openID := getString(senderID, "open_id")
	unionID := getString(senderID, "union_id")
	h.logger.Debug("Sender info - open_id: %s, union_id: %s", openID, unionID)

	if openID == "" {
		h.logger.Debug("No open_id found in sender")
		w.Write([]byte("ok"))
		return
	}

	// Get message content (JSON string)
	content := getString(message, "content")
	if content == "" {
		h.logger.Debug("No content found in message")
		w.Write([]byte("ok"))
		return
	}
	h.logger.Debug("Raw content: %s", content)

	// Parse content JSON
	var contentObj map[string]interface{}
	if err := json.Unmarshal([]byte(content), &contentObj); err != nil {
		h.logger.Error("Failed to parse message content: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Extract text
	text := getString(contentObj, "text")
	if text == "" {
		h.logger.Debug("No text found in content, content keys: %v", getObjectKeys(contentObj))
		w.Write([]byte("ok"))
		return
	}
	h.logger.Debug("Extracted text: '%s'", text)

	// Extract thread info and chat type
	threadID := getString(message, "thread_id")
	h.logger.Debug("Chat type: %s, thread_id: %s", chatType, threadID)

	// Prepare history for AI
	var historyMsgs []domain.AIMessage
	var firstMentioned bool
	botName := h.config.BotName

	// Handle different chat types
	switch chatType {
	case "p2p":
		// Private chat - no mention requirement
		h.logger.Debug("Private chat detected, processing directly")
	case "group", "pgroup", "sgroup":
		h.logger.Debug("Group chat detected, checking mentions or thread context")

		mentioned, newText := h.checkAndStripMention(text, message, botName)
		text = newText

		// Try loading full thread history when thread_id exists
		if threadID != "" {
			threadMessages, err := h.feishuService.ListMessagesByThread(threadID)
			if err != nil {
				h.logger.Error("List thread messages failed: %v", err)
			} else {
				firstMentioned = h.firstMessageMentionsBot(threadMessages, botName)
				historyMsgs = h.buildAIHistoryFromThread(threadMessages, botName)
				h.logger.Debug("Loaded %d messages for history, firstMentioned=%v", len(historyMsgs), firstMentioned)
			}
		}

		if !mentioned && !firstMentioned {
			h.logger.Debug("Bot not mentioned and thread does not start with bot mention, skipping message")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
			return
		}

		h.logger.Debug("Bot mention validated, final text: '%s'", text)
	default:
		h.logger.Debug("Unknown chat type '%s', still processing", chatType)
	}

	// Extract message_id for threading
	messageID := getString(message, "message_id")
	h.logger.Debug("Message ID: %s", messageID)

	// If we already built history, ensure latest user message text matches incoming text
	if len(historyMsgs) > 0 && historyMsgs[len(historyMsgs)-1].Role != "assistant" {
		// Replace last content with cleaned text to avoid mention key residue
		historyMsgs[len(historyMsgs)-1].Content = text
	}

	// Process the message
	h.logger.Debug("Processing message for open_id: %s, text: '%s'", openID, text)
	go h.processMessage(openID, text, messageID, historyMsgs)

	h.logger.Debug("=== IM message queued for processing ===")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("success"))
}
