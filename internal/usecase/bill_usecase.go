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
	// If category is not provided, use default
	if category == nil || *category == "" {
		defaultCat := "其他"
		category = &defaultCat
	}

	// Generate bill ID
	billID := fmt.Sprintf("%s_%d_%d", userName, time.Now().Unix(), rand.Int63n(1000))

	// Set date to now if not provided
	if date == nil {
		now := time.Now()
		date = &now
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

	if err := u.billRepo.CreateBill(bill); err != nil {
		return nil, fmt.Errorf("failed to create bill: %v", err)
	}

	u.logger.Info("Created bill: ID=%s, Description=%s, Amount=%.2f, Category=%s, UserName=%s", bill.ID, bill.Description, bill.Amount, bill.Category, bill.UserName)
	return bill, nil
}

// GetBill retrieves a bill by ID
func (u *BillUseCaseImpl) GetBill(id string) (*domain.Bill, error) {
	return u.billRepo.GetBill(id)
}

// UpdateBill updates a bill
func (u *BillUseCaseImpl) UpdateBill(id string, updates map[string]interface{}) (*domain.Bill, error) {
	bill, err := u.billRepo.GetBill(id)
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

	// bill.UpdatedAt = time.Now()  // Removed: Bill struct doesn't have UpdatedAt field

	if err := u.billRepo.UpdateBill(bill); err != nil {
		return nil, fmt.Errorf("failed to update bill: %v", err)
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

// SuggestCategory suggests category for a bill description
func (u *BillUseCaseImpl) SuggestCategory(userID string, description string) ([]string, error) {
	// TODO: Implement AI-based category suggestion
	// For now, return empty suggestions
	return []string{}, nil
}