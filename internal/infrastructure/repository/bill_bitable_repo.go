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
	// Parse the bitable URL to extract node/app token and table id
	rawToken, tableID, isWiki, err := parseBitableURL(config.BitableURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse bitable URL: %v", err)
	}

	appToken := rawToken
	if isWiki {
		// 当 URL 是 wiki 链接时，需要先通过 node_token 换取真正的 bitable app_token
		appToken, err = feishuService.GetBitableAppTokenFromWikiNode(rawToken)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve bitable app token from wiki node: %v", err)
		}
	}

	return &bitableBillRepository{
		feishuService: feishuService,
		config:        config,
		logger:        logger.GetLogger(),
		appToken:      appToken,
		tableID:       tableID,
	}, nil
}

// parseBitableURL parses the bitable URL to extract token (node_token or app_token) and table id,
// and returns whether this is a wiki node link.
// 支持两种格式：
// 1) base 链接: https://xxx.feishu.cn/base/APP_TOKEN?table=TABLE_ID
// 2) wiki 链接: https://xxx.feishu.cn/wiki/NODE_TOKEN?table=TABLE_ID&view=...
func parseBitableURL(bitableURL string) (token string, tableID string, isWiki bool, err error) {
	if bitableURL == "" {
		return "", "", false, fmt.Errorf("bitable URL is empty")
	}

	// Parse URL
	u, err := url.Parse(bitableURL)
	if err != nil {
		return "", "", false, fmt.Errorf("invalid URL: %v", err)
	}

	// Extract token from path
	pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(pathParts) < 2 || (pathParts[0] != "base" && pathParts[0] != "wiki") {
		return "", "", false, fmt.Errorf("invalid bitable URL format, expected: https://example.feishu.cn/base/APP_TOKEN?table=TABLE_ID or https://example.feishu.cn/wiki/NODE_TOKEN?table=TABLE_ID")
	}
	token = pathParts[1]
	isWiki = pathParts[0] == "wiki"

	// Extract table id from query parameters
	tableID = u.Query().Get("table")
	if tableID == "" {
		return "", "", isWiki, fmt.Errorf("table id not found in URL")
	}

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

	fields := map[string]interface{}{
		r.config.FieldDescription: bill.Description,
		r.config.FieldAmount:      bill.Amount,
		r.config.FieldType:        billType,
		r.config.FieldCategory:    bill.Category,
		r.config.FieldDate:        bill.Date.Format("2006-01-02 15:04:05"),
		r.config.FieldUserName:    bill.UserName,
	}

	// Add original message if configured
	if r.config.FieldOriginalMsg != "" && bill.OriginalMsg != "" {
		fields[r.config.FieldOriginalMsg] = bill.OriginalMsg
	}

	recordID, err := r.feishuService.AddRecordToBitable(
		r.appToken,
		r.tableToken,
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
			"field_name": r.config.FieldType,
			"operator":   "is",
			"value":      []string{typeStr},
		})
	}

	if category != nil && *category != "" {
		filterConditions = append(filterConditions, map[string]interface{}{
			"field_name": r.config.FieldCategory,
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
		Category:    getStringField(fields, r.config.FieldCategory),
		UserName:    getStringField(fields, r.config.FieldUserName),
		OriginalMsg: getStringField(fields, r.config.FieldOriginalMsg),
	}

	// Parse date
	if dateStr := getStringField(fields, r.config.FieldDate); dateStr != "" {
		if t, err := time.Parse("2006-01-02 15:04:05", dateStr); err == nil {
			bill.Date = t
		} else if t, err := time.Parse("2006-01-02", dateStr); err == nil {
			// Backward compatibility: also try parsing date-only format
			bill.Date = t
		}
	}

	// Parse bill type from Chinese
	if typeStr := getStringField(fields, r.config.FieldType); typeStr != "" {
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
