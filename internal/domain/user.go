package domain

// Platform constants for different IM platforms
type Platform string

const (
	PlatformFeishu Platform = "feishu"
	PlatformWechat Platform = "wechat"
	PlatformQQ     Platform = "qq"
)

// UserMapping represents a mapping between platform user ID and user name
type UserMapping struct {
	PlatformID string `json:"open_id"`  // Open ID from platform (e.g., Feishu)
	UserName   string `json:"user_name"` // User's display name
}

// UserMappingRepository interface for user mapping access
type UserMappingRepository interface {
	// GetUserName gets user name by open ID
	GetUserName(openID string) (string, error)

	// SetUserName sets user name for open ID
	SetUserName(openID, userName string) error
}