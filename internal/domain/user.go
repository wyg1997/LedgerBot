package domain

import (
	"time"
)

// Platform constants for different IM platforms
type Platform string

const (
	PlatformFeishu Platform = "feishu"
	PlatformWechat Platform = "wechat"
	PlatformQQ     Platform = "qq"
)

// User represents a user in the system
type User struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	PlatformID string    `json:"platform_id"`
	Platform   Platform  `json:"platform"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// UserMapping represents a mapping between platform user ID and system user
type UserMapping struct {
	Platform   Platform `json:"platform"`
	PlatformID string   `json:"platform_id"`
	UserID     string   `json:"user_id"`
	UserName   string   `json:"user_name"`
}

// UserRepository interface for user data access
type UserRepository interface {
	// GetUserByPlatformID gets a user by platform and platform ID
	GetUserByPlatformID(platform Platform, platformID string) (*User, error)

	// GetUserByID gets a user by system ID
	GetUserByID(id string) (*User, error)

	// CreateUser creates a new user
	CreateUser(user *User) error

	// UpdateUser updates user information
	UpdateUser(user *User) error

	// DeleteUser deletes a user
	DeleteUser(id string) error

	// ListUsers lists all users
	ListUsers() ([]*User, error)
}

// UserMappingRepository interface for user mapping access
type UserMappingRepository interface {
	// GetMapping gets user mapping by platform ID
	GetMapping(platform Platform, platformID string) (*UserMapping, error)

	// CreateMapping creates a new user mapping
	CreateMapping(mapping *UserMapping) error

	// UpdateMapping updates user mapping
	UpdateMapping(mapping *UserMapping) error

	// DeleteMapping deletes user mapping
	DeleteMapping(platform Platform, platformID string) error

	// ListMappings list all mappings
	ListMappings() ([]*UserMapping, error)
}