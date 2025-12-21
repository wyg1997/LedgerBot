package domain

import (
	"time"
)

// BillType represents the type of bill
type BillType string

const (
	BillTypeIncome BillType = "Income" // 收入
	BillTypeExpense BillType = "Expense" // 支出
)

// Bill represents an accounting record
type Bill struct {
	ID          string    `json:"id"`
	Description string    `json:"description"` // 账单描述，如 "午饭"
	Amount      float64   `json:"amount"`      // 金额
	Type        BillType  `json:"type"`        // 收入或支出
	Category    string    `json:"category"`    // 分类，如 "餐饮"
	Date        time.Time `json:"date"`        // 日期
	UserName    string    `json:"user_name"`   // 用户姓名（来自映射）
	OriginalMsg string    `json:"original_msg,omitempty"` // 用户原始消息
	RecordID    string    `json:"record_id,omitempty"`    // 存储系统的记录ID（如 Bitable 的 record_id）
}

// BillRepository interface for bill data access
type BillRepository interface {
	// CreateBill creates a new bill
	CreateBill(bill *Bill) error

	// GetBill gets a bill by ID
	GetBill(id string) (*Bill, error)

	// UpdateBill updates a bill
	UpdateBill(bill *Bill) error

	// DeleteBill deletes a bill
	DeleteBill(id string) error

	// ListBills list bills with pagination and filtering
	ListBills(userName string, startDate, endDate *time.Time, billType *BillType, category *string, offset, limit int) ([]*Bill, int, error)

	// GetMonthlySummary gets monthly summary for a user
	GetMonthlySummary(userName string, year, month int) (*MonthlySummary, error)

	// GetCategories gets all categories for a user
	GetCategories(userName string) ([]string, error)
}

// MonthlySummary represents monthly financial summary
type MonthlySummary struct {
	Year          int     `json:"year"`
	Month         int     `json:"month"`
	TotalIncome   float64 `json:"total_income"`
	TotalExpense  float64 `json:"total_expense"`
	NetAmount     float64 `json:"net_amount"`
	Count         int     `json:"count"`
}

// BillUseCase defines the business logic for bills
type BillUseCase interface {
	// CreateBill creates a new bill with AI categorization if needed
	CreateBill(userName string, userID string, originalMsg string, description string, amount float64, billType BillType, date *time.Time, category *string) (*Bill, error)

	// GetBill retrieves a bill by ID
	GetBill(id string) (*Bill, error)

	// UpdateBill updates a bill
	UpdateBill(id string, updates map[string]interface{}) (*Bill, error)

	// DeleteBill deletes a bill
	DeleteBill(id string) error

	// ListUserBills lists bills for a user with filtering
	ListUserBills(userName string, startDate, endDate *time.Time, billType *BillType, category *string, offset, limit int) ([]*Bill, int, error)

	// GetMonthlySummary gets monthly summary for a user
	GetMonthlySummary(userName string, year, month int) (*MonthlySummary, error)

	// SuggestCategory suggests category for a bill description
	SuggestCategory(userName string, description string) ([]string, error)
}

// CategorySuggestion represents category suggestion from AI
type CategorySuggestion struct {
	Primary   string   `json:"primary"`
	Secondary []string `json:"secondary"`
	Reason    string   `json:"reason"`
}