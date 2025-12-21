package usecase

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/wyg1997/LedgerBot/internal/domain"
	"github.com/wyg1997/LedgerBot/pkg/logger"
)

// BillUseCaseImpl implements BillUseCase
type BillUseCaseImpl struct {
	billRepo       domain.BillRepository
	userMappingRepo domain.UserMappingRepository
	logger         logger.Logger
}

// NewBillUseCase creates a new bill use case
func NewBillUseCase(
	billRepo domain.BillRepository,
	userMappingRepo domain.UserMappingRepository,
) domain.BillUseCase {
	return &BillUseCaseImpl{
		billRepo:        billRepo,
		userMappingRepo: userMappingRepo,
		logger:          logger.GetLogger(),
	}
}

// CreateBill creates a new bill with AI categorization if needed
func (u *BillUseCaseImpl) CreateBill(userName string, userID string, originalMsg string, description string, amount float64, billType domain.BillType, date *time.Time, category *string) (*domain.Bill, error) {
	u.logger.Info("BillUseCase.CreateBill called: userName=%s, userID=%s, description=%s, amount=%.2f, billType=%s, category=%v, originalMsg=%s",
		userName, userID, description, amount, billType, category, originalMsg)

	// If category is not provided, use default
	if category == nil || *category == "" {
		defaultCat := "其他"
		category = &defaultCat
		u.logger.Info("Category not provided, using default: %s", defaultCat)
	}

	// Generate bill ID
	billID := fmt.Sprintf("%s_%d_%d", userName, time.Now().Unix(), rand.Int63n(1000))

	// Set date to now if not provided
	if date == nil {
		now := time.Now()
		date = &now
		u.logger.Info("Date not provided, using current time: %s", date.Format(time.RFC3339))
	}

	bill := &domain.Bill{
		ID:          billID,
		Description: description,
		Amount:      amount,
		Type:        billType,
		Category:    *category,
		Date:        *date,
		UserName:    userName,
		OriginalMsg: originalMsg,
	}

	u.logger.Info("Calling billRepo.CreateBill: billID=%s, description=%s, amount=%.2f, type=%s, category=%s, userName=%s, date=%s",
		bill.ID, bill.Description, bill.Amount, bill.Type, bill.Category, bill.UserName, bill.Date.Format(time.RFC3339))

	if err := u.billRepo.CreateBill(bill); err != nil {
		u.logger.Error("billRepo.CreateBill failed: %v, billID=%s, description=%s, amount=%.2f, type=%s, category=%s, userName=%s",
			err, bill.ID, bill.Description, bill.Amount, bill.Type, bill.Category, bill.UserName)
		return nil, fmt.Errorf("failed to create bill: %v", err)
	}

	u.logger.Info("Bill created successfully: ID=%s, Description=%s, Amount=%.2f, Category=%s, UserName=%s, OriginalMsg=%s",
		bill.ID, bill.Description, bill.Amount, bill.Category, bill.UserName, bill.OriginalMsg)
	return bill, nil
}

// GetBill retrieves a bill by ID
func (u *BillUseCaseImpl) GetBill(id string) (*domain.Bill, error) {
	return u.billRepo.GetBill(id)
}

// UpdateBill updates a bill
// If id starts with "rec" (record_id format), it will update directly without querying
func (u *BillUseCaseImpl) UpdateBill(id string, updates map[string]interface{}) (*domain.Bill, error) {
	var bill *domain.Bill
	
	// If id is a record_id (starts with "rec"), update directly without querying
	// This avoids the need to implement ListRecordsWithFilter for simple updates
	if len(id) >= 3 && id[:3] == "rec" {
		// Direct update by record_id - construct bill with only fields to update
		bill = &domain.Bill{
			ID:       id,
			RecordID: id,
		}
		
		// Apply updates to bill object (only non-empty values)
		if desc, ok := updates["description"].(string); ok && desc != "" {
			bill.Description = desc
		}
		if amount, ok := updates["amount"].(float64); ok && amount > 0 {
			bill.Amount = amount
		}
		if category, ok := updates["category"].(string); ok && category != "" {
			bill.Category = category
		}
		if date, ok := updates["date"].(*time.Time); ok && date != nil {
			bill.Date = *date
		}
		if billType, ok := updates["type"].(domain.BillType); ok && billType != "" {
			bill.Type = billType
		}
		if originalMsg, ok := updates["original_message"].(string); ok && originalMsg != "" {
			bill.OriginalMsg = originalMsg
		}
	} else {
		// Traditional flow: get bill first, then update
		var err error
		bill, err = u.billRepo.GetBill(id)
		if err != nil {
			return nil, err
		}

		// Apply updates
		if desc, ok := updates["description"].(string); ok {
			bill.Description = desc
		}
		if amount, ok := updates["amount"].(float64); ok {
			bill.Amount = amount
		}
		if category, ok := updates["category"].(string); ok {
			bill.Category = category
		}
		if date, ok := updates["date"].(*time.Time); ok {
			bill.Date = *date
		}
		if billType, ok := updates["type"].(domain.BillType); ok {
			bill.Type = billType
		}
		if originalMsg, ok := updates["original_message"].(string); ok {
			bill.OriginalMsg = originalMsg
		}
	}

	// Update through repository (supports partial updates)
	if err := u.billRepo.UpdateBill(bill); err != nil {
		return nil, fmt.Errorf("failed to update bill: %v", err)
	}

	// Ensure RecordID is set for return value
	if bill.RecordID == "" {
		bill.RecordID = id
	}

	return bill, nil
}

// DeleteBill deletes a bill
func (u *BillUseCaseImpl) DeleteBill(id string) error {
	return u.billRepo.DeleteBill(id)
}

// ListUserBills lists bills for a user with filtering
func (u *BillUseCaseImpl) ListUserBills(userID string, startDate, endDate *time.Time, billType *domain.BillType, category *string, offset, limit int) ([]*domain.Bill, int, error) {
	return u.billRepo.ListBills(userID, startDate, endDate, billType, category, offset, limit)
}

// GetMonthlySummary gets monthly summary for a user
func (u *BillUseCaseImpl) GetMonthlySummary(userID string, year, month int) (*domain.MonthlySummary, error) {
	return u.billRepo.GetMonthlySummary(userID, year, month)
}

// QueryTransactions queries transactions within a time range
func (u *BillUseCaseImpl) QueryTransactions(userName string, startTime, endTime time.Time, topN int) ([]*domain.Bill, float64, float64, error) {
	return u.billRepo.QueryTransactions(userName, startTime, endTime, topN)
}

// SuggestCategory suggests category for a bill description
func (u *BillUseCaseImpl) SuggestCategory(userID string, description string) ([]string, error) {
	// TODO: Implement AI-based category suggestion
	// For now, return empty suggestions
	return []string{}, nil
}