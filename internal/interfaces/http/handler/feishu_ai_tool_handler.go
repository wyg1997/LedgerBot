package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

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

	event := getMap(payload, "event")
	if event == nil {
		h.logger.Debug("No event found in payload")
		w.Write([]byte("ok"))
		return
	}

	// Log event details
	h.logger.Debug("Event type: %v", payload["type"])
	h.logger.Debug("Event open_id: %s", getString(event, "open_id"))
	h.logger.Debug("Event text: %s", getString(event, "text"))
	h.logger.Debug("Event chat_type: %s", getString(event, "chat_type"))

	openID := getString(event, "open_id")
	text := getString(event, "text")
	if openID == "" || text == "" {
		w.Write([]byte("ok"))
		return
	}

	// Skip group without mention
	if getString(event, "chat_type") == "group" && !getBool(event, "is_mention") {
		h.logger.Debug("Skipping group message without mention")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
		return
	}

	// Extract without @mention
	if isMention := getBool(event, "is_mention"); isMention {
		if textWO := getString(event, "text_without_at_bot"); textWO != "" {
			text = textWO
		}
	}

	// Log processing decision
	h.logger.Debug("Processing message asynchronously for open_id: %s", openID)

	// Process
	go h.processMessage(openID, text)

	h.logger.Debug("=== Webhook request processed successfully ===")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("success"))
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