package usecase

import (
	"fmt"

	"github.com/wyg1997/LedgerBot/internal/domain"
	"github.com/wyg1997/LedgerBot/pkg/logger"
)

// UserUseCase defines business logic for user operations
type UserUseCase interface {
	// RenameUser updates user's name
	RenameUser(platform domain.Platform, platformID string, newName string) error

	// GetUser gets user name by platform and platform ID
	GetUser(platform domain.Platform, platformID string) (string, error)
}

// UserUseCaseImpl implements UserUseCase
type UserUseCaseImpl struct {
	userMappingRepo domain.UserMappingRepository
	logger          logger.Logger
}

// NewUserUseCase creates a new user use case
func NewUserUseCase(
	userMappingRepo domain.UserMappingRepository,
) UserUseCase {
	return &UserUseCaseImpl{
		userMappingRepo: userMappingRepo,
		logger:          logger.GetLogger(),
	}
}

// RenameUser updates user's name
func (u *UserUseCaseImpl) RenameUser(platform domain.Platform, platformID string, newName string) error {
	// Set user name directly
	if err := u.userMappingRepo.SetUserName(platformID, newName); err != nil {
		return fmt.Errorf("failed to update user name: %v", err)
	}

	u.logger.Info("User renamed: platform=%s, platform_id=%s, new_name=%s", platform, platformID, newName)
	return nil
}

// GetUser gets user name by platform and platform ID
func (u *UserUseCaseImpl) GetUser(platform domain.Platform, platformID string) (string, error) {
	// Get user name from mapping
	userName, err := u.userMappingRepo.GetUserName(platformID)
	if err != nil {
		return "", fmt.Errorf("user mapping not found: %v", err)
	}

	return userName, nil
}