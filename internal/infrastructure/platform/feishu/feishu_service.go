package feishu

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/larksuite/oapi-sdk-go/v3"
	larkbitable "github.com/larksuite/oapi-sdk-go/v3/service/bitable/v1"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkwiki "github.com/larksuite/oapi-sdk-go/v3/service/wiki/v2"
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

	// Create a map with the text content and marshal it to JSON
	messageMap := map[string]string{"text": content}
	textContent, err := json.Marshal(messageMap)
	if err != nil {
		return fmt.Errorf("failed to marshal message content: %v", err)
	}

	// Create reply request
	req := larkim.NewReplyMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			Content(string(textContent)).
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
		s.log.Error("Reply error: %s, code: %s", resp.Code, resp.Msg)
		return fmt.Errorf("failed to reply message: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	s.log.Debug("Successfully replied to message %s", messageID)
	return nil
}

// ListMessagesByThread 查询指定 thread 下的历史消息（按创建时间升序）
func (s *FeishuService) ListMessagesByThread(threadID string) ([]*larkim.Message, error) {
	req := larkim.NewListMessageReqBuilder().
		ContainerIdType("thread").
		ContainerId(threadID).
		SortType("ByCreateTimeAsc").
		PageSize(50).
		Build()

	resp, err := s.client.Im.V1.Message.List(s.ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list thread messages: %w", err)
	}
	if !resp.Success() {
		return nil, fmt.Errorf("list thread messages failed: code=%d msg=%s", resp.Code, resp.Msg)
	}

	if resp.Data == nil || len(resp.Data.Items) == 0 {
		return []*larkim.Message{}, nil
	}

	return resp.Data.Items, nil
}

// SendMessage sends a message to a user
func (s *FeishuService) SendMessage(openID string, content string) error {
	s.log.Debug("Will send message: %s to %s", content, openID)

	// Create a map with the text content and marshal it to JSON
	messageMap := map[string]string{"text": content}
	textContent, err := json.Marshal(messageMap)
	if err != nil {
		return fmt.Errorf("failed to marshal message content: %v", err)
	}

	// Create message request
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("open_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(openID).
			Content(string(textContent)).
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

// AddRecordToBitable 使用 Bitable SDK 创建记录
func (s *FeishuService) AddRecordToBitable(appToken, tableID string, fields map[string]interface{}) (string, error) {
	req := larkbitable.NewCreateAppTableRecordReqBuilder().
		AppToken(appToken).
		TableId(tableID).
		AppTableRecord(larkbitable.NewAppTableRecordBuilder().
			Fields(fields).
			Build()).
		Build()

	resp, err := s.client.Bitable.V1.AppTableRecord.Create(s.ctx, req)
	if err != nil {
		return "", fmt.Errorf("create bitable record failed: %w", err)
	}

	if !resp.Success() {
		return "", fmt.Errorf("create bitable record failed: code=%d msg=%s", resp.Code, resp.Msg)
	}

	if resp.Data == nil || resp.Data.Record == nil || resp.Data.Record.RecordId == nil {
		return "", fmt.Errorf("create bitable record success but record_id is empty")
	}

	recordID := *resp.Data.Record.RecordId
	s.log.Debug("Created bitable record: RecordID=%s, AppToken=%s, TableID=%s", recordID, appToken, tableID)
	return recordID, nil
}

func (s *FeishuService) ListRecords(appToken, tableToken string, pageSize, pageToken int) ([]map[string]interface{}, error) {
	// TODO: Implement with SDK
	return nil, fmt.Errorf("ListRecords not yet implemented with SDK")
}

func (s *FeishuService) ListRecordsWithFilter(appToken, tableToken string, filter map[string]interface{}) ([]map[string]interface{}, error) {
	// TODO: Implement with SDK
	return nil, fmt.Errorf("ListRecordsWithFilter not yet implemented with SDK")
}

// GetBitableAppTokenFromWikiNode 根据 wiki node_token 获取对应多维表格的 app_token
// 通过调用 Wiki.V2.Space.GetNode 接口，读取返回的 node.obj_token 作为 app_token
func (s *FeishuService) GetBitableAppTokenFromWikiNode(nodeToken string) (string, error) {
	if nodeToken == "" {
		return "", fmt.Errorf("node token is empty")
	}

	req := larkwiki.NewGetNodeSpaceReqBuilder().
		Token(nodeToken).
		ObjType("wiki").
		Build()

	// 对于自建应用，使用 tenant access token 即可，SDK 会自动处理，无需额外选项
	resp, err := s.client.Wiki.V2.Space.GetNode(s.ctx, req)
	if err != nil {
		return "", fmt.Errorf("get wiki node failed: %w", err)
	}

	if !resp.Success() {
		return "", fmt.Errorf("get wiki node failed: code=%d msg=%s", resp.Code, resp.Msg)
	}

	if resp.Data == nil || resp.Data.Node == nil || resp.Data.Node.ObjToken == nil {
		return "", fmt.Errorf("get wiki node success but obj_token is empty")
	}

	appToken := *resp.Data.Node.ObjToken
	s.log.Debug("Resolved wiki node %s to bitable app_token %s", nodeToken, appToken)
	return appToken, nil
}
