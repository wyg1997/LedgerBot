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

	// GetUser gets user by platform and platform ID
	GetUser(platform domain.Platform, platformID string) (*domain.User, error)
}

// UserUseCaseImpl implements UserUseCase
type UserUseCaseImpl struct {
	userRepo        domain.UserRepository
	userMappingRepo domain.UserMappingRepository
	logger          logger.Logger
}

// NewUserUseCase creates a new user use case
func NewUserUseCase(
	userRepo domain.UserRepository,
	userMappingRepo domain.UserMappingRepository,
) UserUseCase {
	return &UserUseCaseImpl{
		userRepo:        userRepo,
		userMappingRepo: userMappingRepo,
		logger:          logger.GetLogger(),
	}
}

// RenameUser updates user's name
func (u *UserUseCaseImpl) RenameUser(platform domain.Platform, platformID string, newName string) error {
	// Get user mapping
	mapping, err := u.userMappingRepo.GetMapping(platform, platformID)
	if err != nil {
		return fmt.Errorf("user mapping not found: %v", err)
	}

	// Update mapping user name
	mapping.UserName = newName
	if err := u.userMappingRepo.UpdateMapping(mapping); err != nil {
		return fmt.Errorf("failed to update user mapping: %v", err)
	}

	// TODO: If userRepo is implemented, update the user entity too
	u.logger.Info("User renamed: platform=%s, platform_id=%s, new_name=%s", platform, platformID, newName)
	return nil
}

// GetUser gets user by platform and platform ID
func (u *UserUseCaseImpl) GetUser(platform domain.Platform, platformID string) (*domain.User, error) {
	// First try to get from mapping
	mapping, err := u.userMappingRepo.GetMapping(platform, platformID)
	if err != nil {
		return nil, fmt.Errorf("user mapping not found: %v", err)
	}

	// For now, we don't have full user repository, so create from mapping
	return &domain.User{
		ID:         mapping.UserID,
		Name:       mapping.UserName,
		PlatformID: mapping.PlatformID,
		Platform:   mapping.Platform,
	}, nil
}