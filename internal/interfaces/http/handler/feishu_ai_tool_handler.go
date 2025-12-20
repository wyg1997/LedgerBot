package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

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
func (h *FeishuHandlerAITools) ExecuteFunc(openID string, mapping *domain.UserMapping) func(string, string, domain.BillUseCase, func(string) error) (string, error) {
	return func(input string, userName string, billUseCase domain.BillUseCase, renameFunc func(string) error) (string, error) {
		// Create bill service wrapper
		billService := ai.NewBillService(billUseCase, mapping.UserID, userName)
		// Create rename service wrapper
		renameService := ai.NewRenameService(openID, renameFunc)

		// Call the proper Execute method
		return h.aiservice.Execute(input, userName, billService, renameService)
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

func (h *FeishuHandlerAITools) processMessage(openID, text string) {
	h.logger.Info("Processing from %s: %s", openID, text)

	mapping, err := h.ensureUser(openID)
	if err != nil {
		h.feishuService.SendMessage(openID, "获取用户信息失败")
		return
	}

	// Rename function
	renameFunc := func(name string) error {
		mapping.UserName = name
		return h.userMappingRepo.UpdateMapping(mapping)
	}

	// Execute via tool service
	toolService := h.ExecuteFunc(openID, mapping)
	response, err := toolService(text, mapping.UserName, h.billUseCase, renameFunc)
	if err != nil {
		h.logger.Error("AI execution: %v", err)
		h.feishuService.SendMessage(openID, "处理失败，请重试")
		return
	}

	h.feishuService.SendMessage(openID, response)
}

func (h *FeishuHandlerAITools) ensureUser(openID string) (*domain.UserMapping, error) {
	mapping, err := h.userMappingRepo.GetMapping(domain.PlatformFeishu, openID)
	if err == nil {
		return mapping, nil
	}

	info, err := h.feishuService.GetUserInfo(openID)
	if err != nil {
		return nil, fmt.Errorf("get user info: %w", err)
	}

	mapping = &domain.UserMapping{
		Platform:   domain.PlatformFeishu,
		PlatformID: openID,
		UserID:     info.ID,
		UserName:   info.Name,
	}

	return mapping, h.userMappingRepo.CreateMapping(mapping)
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
	sender := getMap(message, "sender")
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

	// Handle different chat types
	h.logger.Debug("Chat type: %s", chatType)
	switch chatType {
	case "p2p":
		// Private chat - no need to check mentions
		h.logger.Debug("Private chat detected, processing directly")
	case "group", "pgroup", "sgroup":
		// Group chat - need to check mentions
		h.logger.Debug("Group chat detected, checking mentions")

		// Get mentions
		mentions := message["mentions"]
		if mentions == nil {
			h.logger.Debug("No mentions field in group message")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
			return
		}

		mentionList, ok := mentions.([]interface{})
		if !ok || len(mentionList) == 0 {
			h.logger.Debug("No mentions or empty mentions array (type: %T)", mentions)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
			return
		}

		h.logger.Debug("Found %d mentions", len(mentionList))

		// Check if bot is mentioned
		mentioned := false
		botName := h.config.BotName
		h.logger.Debug("Bot name configured as: '%s'", botName)

		for _, mention := range mentionList {
			mentionMap, ok := mention.(map[string]interface{})
			if !ok {
				h.logger.Debug("Skipping invalid mention format (type: %T)", mention)
				continue
			}

			// Check if this mention is for our bot
			name := getString(mentionMap, "name")
			mentionKey := getString(mentionMap, "key")
			mentionOpenID := getString(getMap(mentionMap, "id"), "open_id")

			h.logger.Debug("Checking mention - name: '%s', key: '%s', open_id: '%s'", name, mentionKey, mentionOpenID)

			if name == botName {
				mentioned = true
				h.logger.Debug("Bot mentioned! Found mention with name: '%s'", name)

				// Remove mention from text if key exists
				if mentionKey != "" && strings.Contains(text, mentionKey) {
					oldText := text
					text = strings.TrimSpace(strings.Replace(text, mentionKey, "", 1))
					h.logger.Debug("Removed mention key '%s' from text: '%s' -> '%s'", mentionKey, oldText, text)
				}
				break
			}
		}

		if !mentioned {
			h.logger.Debug("Bot not mentioned, skipping message")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
			return
		}

		h.logger.Debug("Bot mentioned, final text after removal: '%s'", text)
	default:
		h.logger.Debug("Unknown chat type '%s', still processing", chatType)
	}

	// Process the message
	h.logger.Debug("Processing message for open_id: %s, text: '%s'", openID, text)
	go h.processMessage(openID, text)

	h.logger.Debug("=== IM message queued for processing ===")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("success"))
}