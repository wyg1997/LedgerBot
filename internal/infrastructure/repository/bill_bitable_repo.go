package repository

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wyg1997/LedgerBot/config"
	"github.com/wyg1997/LedgerBot/internal/domain"
	"github.com/wyg1997/LedgerBot/internal/infrastructure/platform/feishu"
	"github.com/wyg1997/LedgerBot/pkg/logger"
)

// bitableBillRepository implements BillRepository using Feishu bitable as storage
type bitableBillRepository struct {
	feishuService *feishu.FeishuService
	config        *config.FeishuConfig
	logger        logger.Logger
	appToken      string
	tableID       string
}

// NewBitableBillRepository creates a new bitable bill repository
func NewBitableBillRepository(feishuService *feishu.FeishuService, config *config.FeishuConfig) (domain.BillRepository, error) {
	log := logger.GetLogger()
	// Parse the bitable URL to extract node/app token and table id
	rawToken, tableID, isWiki, err := parseBitableURL(config.BitableURL, log)
	if err != nil {
		return nil, fmt.Errorf("failed to parse bitable URL: %v", err)
	}

	appToken := rawToken
	if isWiki {
		// 当 URL 是 wiki 链接时，需要先通过 node_token 换取真正的 bitable app_token
		log.Info("Converting wiki node_token to bitable app_token: node_token=%s", rawToken)
		appToken, err = feishuService.GetBitableAppTokenFromWikiNode(rawToken)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve bitable app token from wiki node: %v", err)
		}
		log.Info("Successfully converted wiki node_token to app_token: node_token=%s -> app_token=%s", rawToken, appToken)
	} else {
		log.Info("Using direct bitable URL, app_token=%s, table_id=%s", appToken, tableID)
	}

	return &bitableBillRepository{
		feishuService: feishuService,
		config:        config,
		logger:        log,
		appToken:      appToken,
		tableID:       tableID,
	}, nil
}

// parseBitableURL parses the bitable URL to extract token (node_token or app_token) and table id,
// and returns whether this is a wiki node link.
// 支持两种格式：
// 1) base 链接: https://xxx.feishu.cn/base/APP_TOKEN?table=TABLE_ID
// 2) wiki 链接: https://xxx.feishu.cn/wiki/NODE_TOKEN?table=TABLE_ID&view=...
func parseBitableURL(bitableURL string, log logger.Logger) (token string, tableID string, isWiki bool, err error) {
	if bitableURL == "" {
		return "", "", false, fmt.Errorf("bitable URL is empty")
	}

	// Remove protocol prefix (https:// or http://) if present
	cleanedURL := bitableURL
	if strings.HasPrefix(cleanedURL, "https://") {
		cleanedURL = strings.TrimPrefix(cleanedURL, "https://")
	} else if strings.HasPrefix(cleanedURL, "http://") {
		cleanedURL = strings.TrimPrefix(cleanedURL, "http://")
	}

	// Split URL into path and query parts
	parts := strings.SplitN(cleanedURL, "?", 2)
	pathPart := parts[0]
	var queryPart string
	if len(parts) > 1 {
		queryPart = parts[1]
	}

	// Parse path: remove leading and trailing slashes, then split
	path := strings.Trim(pathPart, "/")
	if path == "" {
		return "", "", false, fmt.Errorf("empty path in URL: %s", bitableURL)
	}

	// Split path by "/" to get domain and path components
	// Format: domain.com/wiki/TOKEN or domain.com/base/TOKEN
	pathSegments := strings.Split(path, "/")
	if len(pathSegments) < 3 {
		return "", "", false, fmt.Errorf("invalid bitable URL format: path has less than 3 segments (path=%s, segments=%v), expected: example.feishu.cn/base/APP_TOKEN?table=TABLE_ID or example.feishu.cn/wiki/NODE_TOKEN?table=TABLE_ID", path, pathSegments)
	}

	// Find "base" or "wiki" in path segments
	var baseOrWikiIndex = -1
	for i, segment := range pathSegments {
		if segment == "base" || segment == "wiki" {
			baseOrWikiIndex = i
			break
		}
	}

	if baseOrWikiIndex == -1 {
		return "", "", false, fmt.Errorf("invalid bitable URL format: 'base' or 'wiki' not found in path (path=%s, segments=%v)", path, pathSegments)
	}

	if baseOrWikiIndex+1 >= len(pathSegments) {
		return "", "", false, fmt.Errorf("invalid bitable URL format: token not found after 'base' or 'wiki' (path=%s)", path)
	}

	firstPart := pathSegments[baseOrWikiIndex]
	token = pathSegments[baseOrWikiIndex+1]
	if token == "" {
		return "", "", false, fmt.Errorf("empty token in URL path (path=%s)", path)
	}

	isWiki = firstPart == "wiki"

	// Parse query parameters to get table id
	if queryPart != "" {
		queryParams, err := url.ParseQuery(queryPart)
		if err != nil {
			return "", "", isWiki, fmt.Errorf("invalid query parameters: %v", err)
		}
		tableID = queryParams.Get("table")
	}

	if tableID == "" {
		return "", "", isWiki, fmt.Errorf("table id not found in URL query parameters")
	}

	log.Debug("parseBitableURL: input=%s, result: token=%s, tableID=%s, isWiki=%v", bitableURL, token, tableID, isWiki)
	return token, tableID, isWiki, nil
}

// CreateBill creates a new bill in bitable
func (r *bitableBillRepository) CreateBill(bill *domain.Bill) error {
	if bill.ID == "" {
		bill.ID = fmt.Sprintf("%s_%d", bill.UserName, time.Now().Unix())
	}

	// Convert type to Chinese
	billType := "支出"
	if bill.Type == domain.BillTypeIncome {
		billType = "收入"
	}

	// 日期需要转换为毫秒时间戳
	dateTimestamp := bill.Date.UnixMilli()

	fields := map[string]interface{}{
		r.config.FieldDescription: bill.Description,
		r.config.FieldAmount:      bill.Amount,
		r.config.FieldType:        bill.Category,
		r.config.FieldCategory:    billType,
		r.config.FieldDate:        dateTimestamp,
		r.config.FieldUserName:    bill.UserName,
	}

	// Add original message if configured
	if r.config.FieldOriginalMsg != "" {
		if bill.OriginalMsg != "" {
			fields[r.config.FieldOriginalMsg] = bill.OriginalMsg
			r.logger.Debug("Added original message to fields: field=%s, value=%s", r.config.FieldOriginalMsg, bill.OriginalMsg)
		} else {
			r.logger.Debug("Original message field is configured but bill.OriginalMsg is empty")
		}
	} else {
		if bill.OriginalMsg != "" {
			r.logger.Debug("Original message exists but field name is not configured: OriginalMsg=%s", bill.OriginalMsg)
		}
	}

	r.logger.Debug("Preparing to create bill in bitable: app_token=%s, table_id=%s, fields=%+v", r.appToken, r.tableID, fields)

	recordID, err := r.feishuService.AddRecordToBitable(
		r.appToken,
		r.tableID,
		fields,
	)

	if err != nil {
		r.logger.Error("Failed to create bill in bitable: %v", err)
		return fmt.Errorf("failed to create bill: %v", err)
	}

	r.logger.Info("Created bill in bitable: RecordID=%s, BillID=%s", recordID, bill.ID)
	return nil
}

// GetBill gets a bill by ID from bitable
func (r *bitableBillRepository) GetBill(id string) (*domain.Bill, error) {
	// For bitable, we need to query by bill ID field
	// This requires implementing query functionality in FeishuService
	bills, _, err := r.ListBills("", nil, nil, nil, nil, 0, 100) // Get all and filter
	if err != nil {
		return nil, err
	}

	for _, bill := range bills {
		if bill.ID == id {
			return bill, nil
		}
	}

	return nil, fmt.Errorf("bill not found: %s", id)
}

// UpdateBill updates a bill in bitable
func (r *bitableBillRepository) UpdateBill(bill *domain.Bill) error {
	// In bitable, we would need to:
	// 1. Find the record by bill ID
	// 2. Update the record with new values
	// This requires implementing update functionality in FeishuService
	return fmt.Errorf("update bill not implemented for bitable storage")
}

// DeleteBill deletes a bill from bitable
func (r *bitableBillRepository) DeleteBill(id string) error {
	// In bitable, we would need to:
	// 1. Find the record by bill ID
	// 2. Delete the record
	// This requires implementing delete functionality in FeishuService
	return fmt.Errorf("delete bill not implemented for bitable storage")
}

// ListBills lists bills with filtering
func (r *bitableBillRepository) ListBills(username string, startDate, endDate *time.Time, billType *domain.BillType, category *string, offset, limit int) ([]*domain.Bill, int, error) {
	// Build filter conditions
	filterConditions := []map[string]interface{}{}

	if username != "" {
		filterConditions = append(filterConditions, map[string]interface{}{
			"field_name": r.config.FieldUserName,
			"operator":   "is",
			"value":      []string{username},
		})
	}

	if billType != nil {
		typeStr := "支出"
		if *billType == domain.BillTypeIncome {
			typeStr = "收入"
		}
		filterConditions = append(filterConditions, map[string]interface{}{
			"field_name": r.config.FieldCategory,
			"operator":   "is",
			"value":      []string{typeStr},
		})
	}

	if category != nil && *category != "" {
		filterConditions = append(filterConditions, map[string]interface{}{
			"field_name": r.config.FieldType,
			"operator":   "is",
			"value":      []string{*category},
		})
	}

	// Date range filter
	if startDate != nil || endDate != nil {
		dateCondition := map[string]interface{}{
			"field_name": r.config.FieldDate,
			"operator":   "is_within",
			"field_type": 5, // Date field type
		}

		if startDate != nil && endDate != nil {
			dateCondition["value"] = []string{
				startDate.Format("2006-01-02 15:04:05"),
				endDate.Format("2006-01-02 15:04:05"),
			}
		}
		filterConditions = append(filterConditions, dateCondition)
	}

	// Build the full filter
	filter := map[string]interface{}{
		"automatic_fields": false,
		"field_names": []string{
			"_id", // record id
			r.config.FieldDescription,
			r.config.FieldAmount,
			r.config.FieldType,
			r.config.FieldCategory,
			r.config.FieldDate,
			r.config.FieldUserName,
			r.config.FieldOriginalMsg,
		},
		"page_size": limit,
	}

	if len(filterConditions) > 0 {
		filter["filter"] = map[string]interface{}{
			"conjunction": "and",
			"conditions":  filterConditions,
		}
	}

	// Query records
	records, err := r.feishuService.ListRecordsWithFilter(
		r.appToken,
		r.tableID,
		filter,
	)

	if err != nil {
		r.logger.Error("Failed to list bills from bitable: %v", err)
		return nil, 0, fmt.Errorf("failed to list bills: %v", err)
	}

	// Convert records to bills
	bills := make([]*domain.Bill, 0, len(records))
	for _, record := range records {
		bill, err := r.convertRecordToBill(record)
		if err != nil {
			r.logger.Error("Failed to convert record to bill: %v", err)
			continue
		}
		bills = append(bills, bill)
	}

	return bills, len(bills), nil
}

// GetMonthlySummary gets monthly summary for a user
func (r *bitableBillRepository) GetMonthlySummary(username string, year, month int) (*domain.MonthlySummary, error) {
	// This would require aggregating data from bitable
	// For now, return empty summary
	r.logger.Warn("GetMonthlySummary not implemented for bitable storage")
	return &domain.MonthlySummary{
		Year:  year,
		Month: month,
	}, nil
}

// GetCategories gets all categories for a user
func (r *bitableBillRepository) GetCategories(userName string) ([]string, error) {
	// This would require querying unique categories from bitable
	// For now, return empty list
	r.logger.Warn("GetCategories not implemented for bitable storage")
	return []string{}, nil
}

// Helper function to convert interface to float64
func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
		return 0
	default:
		return 0
	}
}

// convertRecordToBill converts a bitable record to a Bill
func (r *bitableBillRepository) convertRecordToBill(record map[string]interface{}) (*domain.Bill, error) {
	// Extract record ID
	recordID := ""
	if id, ok := record["_id"].(string); ok {
		recordID = id
	}

	// Extract fields
	fields := make(map[string]interface{})
	if f, ok := record["fields"].(map[string]interface{}); ok {
		fields = f
	}

	// Parse bill data
	bill := &domain.Bill{
		ID:          recordID,
		Description: getStringField(fields, r.config.FieldDescription),
		Amount:      getNumberField(fields, r.config.FieldAmount),
		Category:    getStringField(fields, r.config.FieldType),
		UserName:    getStringField(fields, r.config.FieldUserName),
		OriginalMsg: getStringField(fields, r.config.FieldOriginalMsg),
	}

	// Parse date - 支持毫秒时间戳（新格式）和字符串格式（向后兼容）
	if dateVal, ok := fields[r.config.FieldDate]; ok {
		if dateTimestamp, ok := dateVal.(int64); ok {
			// 毫秒时间戳格式
			bill.Date = time.UnixMilli(dateTimestamp)
		} else if dateTimestamp, ok := dateVal.(float64); ok {
			// 处理 JSON 数字可能被解析为 float64 的情况
			bill.Date = time.UnixMilli(int64(dateTimestamp))
		} else if dateStr, ok := dateVal.(string); ok && dateStr != "" {
			// 向后兼容：字符串格式
			if t, err := time.Parse("2006-01-02 15:04:05", dateStr); err == nil {
				bill.Date = t
			} else if t, err := time.Parse("2006-01-02", dateStr); err == nil {
				bill.Date = t
			}
		}
	}

	// Parse bill type from Chinese (收支类型存储在 FieldCategory)
	if typeStr := getStringField(fields, r.config.FieldCategory); typeStr != "" {
		if typeStr == "收入" {
			bill.Type = domain.BillTypeIncome
		} else {
			bill.Type = domain.BillTypeExpense
		}
	}

	return bill, nil
}

// Helper functions to extract field values
func getStringField(fields map[string]interface{}, fieldName string) string {
	if val, ok := fields[fieldName]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getNumberField(fields map[string]interface{}, fieldName string) float64 {
	if val, ok := fields[fieldName]; ok {
		return toFloat64(val)
	}
	return 0
}
