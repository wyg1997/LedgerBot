package feishu

import (
	"context"
	"fmt"
	"strings"

	"github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/wyg1997/LedgerBot/config"
	"github.com/wyg1997/LedgerBot/pkg/logger"
)

// FeishuService handles Feishu API integration
type FeishuService struct {
	config *config.FeishuConfig
	client *lark.Client
	log    logger.Logger
	ctx    context.Context
}

// NewFeishuService creates a new Feishu service
func NewFeishuService(cfg *config.FeishuConfig) *FeishuService {
	client := lark.NewClient(cfg.AppID, cfg.AppSecret)
	return &FeishuService{
		config: cfg,
		client: client,
		log:    logger.GetLogger(),
		ctx:    context.Background(),
	}
}

// ReplyMessage replies to a message in thread
func (s *FeishuService) ReplyMessage(messageID string, content string, uuid string) error {
    s.log.Debug("Will reply message: %s, message_id: %s", content, messageID)
	// Create text content as JSON string, escape quotes
	escapedContent := strings.ReplaceAll(content, `"`, `\"`)
	textContent := fmt.Sprintf(`{"text":"%s"}`, escapedContent)

	// Create reply request
	req := larkim.NewReplyMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			Content(textContent).
			MsgType("text").
			Uuid(uuid).
			ReplyInThread(true).
			Build()).
		Build()

	// Execute the request
	resp, err := s.client.Im.Message.Reply(s.ctx, req)
	if err != nil {
		return fmt.Errorf("failed to reply message: %v", err)
	}

	// Check response code
	if !resp.Success() {
        s.log.Error("Replay error: %s, code: %s", resp.Code, resp.Msg)
		return fmt.Errorf("failed to reply message: code=%d, msg=%s", resp.Code, resp.Msg)
    }

	s.log.Debug("Successfully replied to message %s", messageID)
	return nil
}

// SendMessage sends a message to a user
func (s *FeishuService) SendMessage(openID string, content string) error {
    s.log.Debug("Will send message: %s to %s", content, openID)
	// Create text content as JSON string, escape quotes
	escapedContent := strings.ReplaceAll(content, `"`, `\"`)
	textContent := fmt.Sprintf(`{"text":"%s"}`, escapedContent)

	// Create message request
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("open_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(openID).
			Content(textContent).
			MsgType("text").
			Build()).
		Build()

	// Execute the request
	resp, err := s.client.Im.Message.Create(s.ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send message: %v", err)
	}

	// Check response code
	if !resp.Success() {
		return fmt.Errorf("failed to send message: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	s.log.Debug("Successfully sent message to user %s", openID)
	return nil
}

// MessageCallback represents callback from Feishu
type MessageCallback struct {
	UUID  string `json:"uuid"`
	Token string `json:"token"`
	Type  string `json:"type"`
	Event struct {
		Type             string `json:"type"`
		AppID            string `json:"app_id"`
		TenantKey        string `json:"tenant_key"`
		OpenID           string `json:"open_id"`
		OpenMessageID    string `json:"open_message_id"`
		IsMention        bool   `json:"is_mention"`
		TextWithoutAtBot string `json:"text_without_at_bot"`
		Text             string `json:"text"`
	} `json:"event"`
}

// ProcessMessageCallback processes incoming message callback
func (s *FeishuService) ProcessMessageCallback(callback MessageCallback) (string, error) {
	// Avoid using contacts API due to permission requirements
	return "success", nil
}

// Placeholder methods for compatibility - these would need to be fully migrated to SDK
func (s *FeishuService) AddRecordToBitable(appToken, tableToken string, fields map[string]interface{}) (string, error) {
	// TODO: Implement with SDK
	return "", fmt.Errorf("AddRecordToBitable not yet implemented with SDK")
}

func (s *FeishuService) ListRecords(appToken, tableToken string, pageSize, pageToken int) ([]map[string]interface{}, error) {
	// TODO: Implement with SDK
	return nil, fmt.Errorf("ListRecords not yet implemented with SDK")
}

func (s *FeishuService) ListRecordsWithFilter(appToken, tableToken string, filter map[string]interface{}) ([]map[string]interface{}, error) {
	// TODO: Implement with SDK
	return nil, fmt.Errorf("ListRecordsWithFilter not yet implemented with SDK")
}
