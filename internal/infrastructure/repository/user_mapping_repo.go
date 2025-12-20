package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/wyg1997/LedgerBot/internal/domain"
)

// userMappingRepository implements UserMappingRepository with file-based storage
type userMappingRepository struct {
	file     string
	mu       sync.RWMutex
	mappings map[string]*domain.UserMapping
}

// NewUserMappingRepository creates a new user mapping repository
func NewUserMappingRepository(file string) (domain.UserMappingRepository, error) {
	repo := &userMappingRepository{
		file:     file,
		mappings: make(map[string]*domain.UserMapping),
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

// GetMapping gets user mapping by platform and platform ID
func (r *userMappingRepository) GetMapping(platform domain.Platform, platformID string) (*domain.UserMapping, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := r.makeKey(platform, platformID)
	mapping, exists := r.mappings[key]
	if !exists {
		return nil, fmt.Errorf("mapping not found for platform %s and ID %s", platform, platformID)
	}

	return mapping, nil
}

// CreateMapping creates a new user mapping
func (r *userMappingRepository) CreateMapping(mapping *domain.UserMapping) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := r.makeKey(mapping.Platform, mapping.PlatformID)

	// Check if already exists
	if _, exists := r.mappings[key]; exists {
		return fmt.Errorf("mapping already exists for platform %s and ID %s", mapping.Platform, mapping.PlatformID)
	}

	// Add to map
	r.mappings[key] = mapping

	// Save to file
	return r.save()
}

// UpdateMapping updates user mapping
func (r *userMappingRepository) UpdateMapping(mapping *domain.UserMapping) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := r.makeKey(mapping.Platform, mapping.PlatformID)

	// Check if exists
	if _, exists := r.mappings[key]; !exists {
		return fmt.Errorf("mapping not found for platform %s and ID %s", mapping.Platform, mapping.PlatformID)
	}

	// Update in map
	r.mappings[key] = mapping

	// Save to file
	return r.save()
}

// DeleteMapping deletes user mapping
func (r *userMappingRepository) DeleteMapping(platform domain.Platform, platformID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := r.makeKey(platform, platformID)

	if _, exists := r.mappings[key]; !exists {
		return fmt.Errorf("mapping not found for platform %s and ID %s", platform, platformID)
	}

	delete(r.mappings, key)

	return r.save()
}

// ListMappings lists all mappings
func (r *userMappingRepository) ListMappings() ([]*domain.UserMapping, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*domain.UserMapping, 0, len(r.mappings))
	for _, mapping := range r.mappings {
		result = append(result, mapping)
	}

	return result, nil
}

// makeKey creates a key for the mapping
func (r *userMappingRepository) makeKey(platform domain.Platform, platformID string) string {
	return string(platform) + ":" + platformID
}

// load loads mappings from file
func (r *userMappingRepository) load() error {
	if r.file == "" {
		return nil
	}

	data, err := os.ReadFile(r.file)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		return nil
	}

	var mappings []*domain.UserMapping
	if err := json.Unmarshal(data, &mappings); err != nil {
		return fmt.Errorf("failed to unmarshal mappings: %v", err)
	}

	// Convert to map
	for _, mapping := range mappings {
		key := r.makeKey(mapping.Platform, mapping.PlatformID)
		r.mappings[key] = mapping
	}

	return nil
}

// save saves mappings to file
func (r *userMappingRepository) save() error {
	if r.file == "" {
		return nil
	}

	// Create directory if needed
	dir := filepath.Dir(r.file)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Convert map to slice
	mappings := make([]*domain.UserMapping, 0, len(r.mappings))
	for _, mapping := range r.mappings {
		mappings = append(mappings, mapping)
	}

	data, err := json.MarshalIndent(mappings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal mappings: %v", err)
	}

	return os.WriteFile(r.file, data, 0644)
}