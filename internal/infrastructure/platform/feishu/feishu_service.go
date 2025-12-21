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
	s.log.Debug("Creating bitable record: app_token=%s, table_id=%s, fields=%+v", appToken, tableID, fields)

	req := larkbitable.NewCreateAppTableRecordReqBuilder().
		AppToken(appToken).
		TableId(tableID).
		AppTableRecord(larkbitable.NewAppTableRecordBuilder().
			Fields(fields).
			Build()).
		Build()

	resp, err := s.client.Bitable.V1.AppTableRecord.Create(s.ctx, req)
	if err != nil {
		s.log.Error("Create bitable record API call failed: app_token=%s, table_id=%s, error=%v", appToken, tableID, err)
		return "", fmt.Errorf("create bitable record failed: %w", err)
	}

	if !resp.Success() {
		s.log.Error("Create bitable record failed: app_token=%s, table_id=%s, code=%d, msg=%s", appToken, tableID, resp.Code, resp.Msg)
		return "", fmt.Errorf("create bitable record failed: code=%d msg=%s", resp.Code, resp.Msg)
	}

	if resp.Data == nil || resp.Data.Record == nil || resp.Data.Record.RecordId == nil {
		s.log.Error("Create bitable record success but record_id is empty: app_token=%s, table_id=%s", appToken, tableID)
		return "", fmt.Errorf("create bitable record success but record_id is empty")
	}

	recordID := *resp.Data.Record.RecordId
	s.log.Debug("Successfully created bitable record: record_id=%s, app_token=%s, table_id=%s", recordID, appToken, tableID)
	return recordID, nil
}

// UpdateRecordToBitable 使用 Bitable SDK 更新记录
func (s *FeishuService) UpdateRecordToBitable(appToken, tableID, recordID string, fields map[string]interface{}) (string, error) {
	s.log.Debug("Updating bitable record: app_token=%s, table_id=%s, record_id=%s, fields=%+v", appToken, tableID, recordID, fields)

	req := larkbitable.NewUpdateAppTableRecordReqBuilder().
		AppToken(appToken).
		TableId(tableID).
		RecordId(recordID).
		AppTableRecord(larkbitable.NewAppTableRecordBuilder().
			Fields(fields).
			Build()).
		Build()

	resp, err := s.client.Bitable.V1.AppTableRecord.Update(s.ctx, req)
	if err != nil {
		s.log.Error("Update bitable record API call failed: app_token=%s, table_id=%s, record_id=%s, error=%v", appToken, tableID, recordID, err)
		return "", fmt.Errorf("update bitable record failed: %w", err)
	}

	if !resp.Success() {
		s.log.Error("Update bitable record failed: app_token=%s, table_id=%s, record_id=%s, code=%d, msg=%s", appToken, tableID, recordID, resp.Code, resp.Msg)
		return "", fmt.Errorf("update bitable record failed: code=%d msg=%s", resp.Code, resp.Msg)
	}

	if resp.Data == nil || resp.Data.Record == nil || resp.Data.Record.RecordId == nil {
		s.log.Error("Update bitable record success but record_id is empty: app_token=%s, table_id=%s, record_id=%s", appToken, tableID, recordID)
		return "", fmt.Errorf("update bitable record success but record_id is empty")
	}

	updatedRecordID := *resp.Data.Record.RecordId
	s.log.Debug("Successfully updated bitable record: record_id=%s, app_token=%s, table_id=%s", updatedRecordID, appToken, tableID)
	return updatedRecordID, nil
}

// BatchGetRecordsToBitable 使用 Bitable SDK 批量获取记录
func (s *FeishuService) BatchGetRecordsToBitable(appToken, tableID string, recordIDs []string) ([]map[string]interface{}, error) {
	s.log.Debug("Batch getting bitable records: app_token=%s, table_id=%s, record_ids=%v", appToken, tableID, recordIDs)

	if len(recordIDs) == 0 {
		return []map[string]interface{}{}, nil
	}

	req := larkbitable.NewBatchGetAppTableRecordReqBuilder().
		AppToken(appToken).
		TableId(tableID).
		Body(larkbitable.NewBatchGetAppTableRecordReqBodyBuilder().
			RecordIds(recordIDs).
			AutomaticFields(true).
			Build()).
		Build()

	resp, err := s.client.Bitable.V1.AppTableRecord.BatchGet(s.ctx, req)
	if err != nil {
		s.log.Error("BatchGet bitable records API call failed: app_token=%s, table_id=%s, record_ids=%v, error=%v", appToken, tableID, recordIDs, err)
		return nil, fmt.Errorf("batch get bitable records failed: %w", err)
	}

	if !resp.Success() {
		s.log.Error("BatchGet bitable records failed: app_token=%s, table_id=%s, record_ids=%v, code=%d, msg=%s", appToken, tableID, recordIDs, resp.Code, resp.Msg)
		return nil, fmt.Errorf("batch get bitable records failed: code=%d msg=%s", resp.Code, resp.Msg)
	}

	if resp.Data == nil || resp.Data.Records == nil {
		s.log.Error("BatchGet bitable records success but records is empty: app_token=%s, table_id=%s, record_ids=%v", appToken, tableID, recordIDs)
		return []map[string]interface{}{}, nil
	}

	// Convert records to maps
	records := make([]map[string]interface{}, 0, len(resp.Data.Records))
	for _, rec := range resp.Data.Records {
		record := make(map[string]interface{})
		if rec.RecordId != nil {
			record["_id"] = *rec.RecordId
		}
		if rec.Fields != nil {
			record["fields"] = rec.Fields
		}
		records = append(records, record)
	}

	s.log.Debug("Successfully batch got bitable records: count=%d, app_token=%s, table_id=%s", len(records), appToken, tableID)
	return records, nil
}

// GetRecordToBitable 使用 Bitable SDK 通过 record_id 获取单条记录（使用 BatchGet）
func (s *FeishuService) GetRecordToBitable(appToken, tableID, recordID string) (map[string]interface{}, error) {
	s.log.Debug("Getting bitable record: app_token=%s, table_id=%s, record_id=%s", appToken, tableID, recordID)

	records, err := s.BatchGetRecordsToBitable(appToken, tableID, []string{recordID})
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("record not found: %s", recordID)
	}

	return records[0], nil
}

// DeleteRecordToBitable 使用 Bitable SDK 删除记录
func (s *FeishuService) DeleteRecordToBitable(appToken, tableID, recordID string) error {
	s.log.Debug("Deleting bitable record: app_token=%s, table_id=%s, record_id=%s", appToken, tableID, recordID)

	req := larkbitable.NewBatchDeleteAppTableRecordReqBuilder().
		AppToken(appToken).
		TableId(tableID).
		Body(larkbitable.NewBatchDeleteAppTableRecordReqBodyBuilder().
			Records([]string{recordID}).
			Build()).
		Build()

	resp, err := s.client.Bitable.V1.AppTableRecord.BatchDelete(s.ctx, req)
	if err != nil {
		s.log.Error("Delete bitable record API call failed: app_token=%s, table_id=%s, record_id=%s, error=%v", appToken, tableID, recordID, err)
		return fmt.Errorf("delete bitable record failed: %w", err)
	}

	if !resp.Success() {
		s.log.Error("Delete bitable record failed: app_token=%s, table_id=%s, record_id=%s, code=%d, msg=%s", appToken, tableID, recordID, resp.Code, resp.Msg)
		return fmt.Errorf("delete bitable record failed: code=%d msg=%s", resp.Code, resp.Msg)
	}

	s.log.Debug("Successfully deleted bitable record: record_id=%s, app_token=%s, table_id=%s", recordID, appToken, tableID)
	return nil
}

func (s *FeishuService) ListRecords(appToken, tableToken string, pageSize, pageToken int) ([]map[string]interface{}, error) {
	// TODO: Implement with SDK
	return nil, fmt.Errorf("ListRecords not yet implemented with SDK")
}

func (s *FeishuService) ListRecordsWithFilter(appToken, tableToken string, filter map[string]interface{}) ([]map[string]interface{}, error) {
	// TODO: Implement with SDK
	return nil, fmt.Errorf("ListRecordsWithFilter not yet implemented with SDK")
}

// SearchRecords 使用 Bitable SDK 搜索记录
func (s *FeishuService) SearchRecords(appToken, tableID string, startTime, endTime int64, fieldNames []string, pageSize int) ([]map[string]interface{}, int, string, error) {
	s.log.Debug("Searching bitable records: app_token=%s, table_id=%s, start_time=%d, end_time=%d, page_size=%d", 
		appToken, tableID, startTime, endTime, pageSize)

	// Build filter conditions for date range
	conditions := []*larkbitable.Condition{
		larkbitable.NewConditionBuilder().
			FieldName(s.config.FieldDate).
			Operator("isGreater").
			Value([]string{"ExactDate", fmt.Sprintf("%d", startTime)}).
			Build(),
		larkbitable.NewConditionBuilder().
			FieldName(s.config.FieldDate).
			Operator("isLess").
			Value([]string{"ExactDate", fmt.Sprintf("%d", endTime)}).
			Build(),
	}

	// Build sort by date descending
	sorts := []*larkbitable.Sort{
		larkbitable.NewSortBuilder().
			FieldName(s.config.FieldDate).
			Desc(true).
			Build(),
	}

	req := larkbitable.NewSearchAppTableRecordReqBuilder().
		AppToken(appToken).
		TableId(tableID).
		PageSize(pageSize).
		Body(larkbitable.NewSearchAppTableRecordReqBodyBuilder().
			FieldNames(fieldNames).
			Sort(sorts).
			Filter(larkbitable.NewFilterInfoBuilder().
				Conjunction("and").
				Conditions(conditions).
				Build()).
			AutomaticFields(false).
			Build()).
		Build()

	resp, err := s.client.Bitable.V1.AppTableRecord.Search(s.ctx, req)
	if err != nil {
		s.log.Error("Search bitable records API call failed: app_token=%s, table_id=%s, error=%v", appToken, tableID, err)
		return nil, 0, "", fmt.Errorf("search bitable records failed: %w", err)
	}

	if !resp.Success() {
		s.log.Error("Search bitable records failed: app_token=%s, table_id=%s, code=%d, msg=%s", appToken, tableID, resp.Code, resp.Msg)
		return nil, 0, "", fmt.Errorf("search bitable records failed: code=%d msg=%s", resp.Code, resp.Msg)
	}

	// Parse response
	var records []map[string]interface{}
	var total int
	var pageToken string

	if resp.Data != nil {
		if resp.Data.HasMore != nil {
			// has_more is available
		}
		if resp.Data.PageToken != nil {
			pageToken = *resp.Data.PageToken
		}
		if resp.Data.Total != nil {
			total = int(*resp.Data.Total)
		}
		if resp.Data.Items != nil {
			for _, item := range resp.Data.Items {
				record := make(map[string]interface{})
				if item.RecordId != nil {
					record["_id"] = *item.RecordId
					record["record_id"] = *item.RecordId
				}
				if item.Fields != nil {
					record["fields"] = item.Fields
				}
				records = append(records, record)
			}
		}
	}

	s.log.Debug("Successfully searched bitable records: count=%d, total=%d, app_token=%s, table_id=%s", len(records), total, appToken, tableID)
	return records, total, pageToken, nil
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
	s.log.Info("Resolved wiki node to bitable app_token: node_token=%s -> app_token=%s", nodeToken, appToken)
	return appToken, nil
}
