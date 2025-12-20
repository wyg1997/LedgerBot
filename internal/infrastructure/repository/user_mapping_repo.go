package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wyg1997/LedgerBot/internal/domain"
)

// userMappingRepository implements UserMappingRepository with file-based storage
type userMappingRepository struct {
	dataDir  string
	mu       sync.RWMutex
	mappings map[string]string // openID -> userName
}

// NewUserMappingRepository creates a new user mapping repository
func NewUserMappingRepository(dataDir string) (domain.UserMappingRepository, error) {
	repo := &userMappingRepository{
		dataDir:  dataDir,
		mappings: make(map[string]string),
	}

	// Try to load from file
	if err := repo.load(); err != nil {
		// If file doesn't exist, return empty repo
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load user mappings: %v", err)
		}
	}

	return repo, nil
}

// GetUserName gets user name by open ID
func (r *userMappingRepository) GetUserName(openID string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	name, exists := r.mappings[openID]
	if !exists {
		return "", fmt.Errorf("user name not found for openID: %s", openID)
	}

	// Validate that the retrieved name is not empty or whitespace-only
	if name == "" || strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("user name is empty or invalid for openID: %s", openID)
	}

	return name, nil
}

// SetUserName sets user name for open ID
func (r *userMappingRepository) SetUserName(openID, userName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate user name is not empty or whitespace
	if userName == "" || strings.TrimSpace(userName) == "" {
		return fmt.Errorf("user name cannot be empty")
	}

	// Update mapping
	r.mappings[openID] = userName

	// Save to file
	return r.save()
}

// load loads mappings from file
func (r *userMappingRepository) load() error {
	filePath := filepath.Join(r.dataDir, "user_mapping.json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		return nil
	}

	return json.Unmarshal(data, &r.mappings)
}

// save saves mappings to file
func (r *userMappingRepository) save() error {
	filePath := filepath.Join(r.dataDir, "user_mapping.json")

	// Create directory if needed
	if err := os.MkdirAll(r.dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	data, err := json.MarshalIndent(r.mappings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal mappings: %v", err)
	}

	return os.WriteFile(filePath, data, 0644)
}
