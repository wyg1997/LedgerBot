package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// LoadEnvFile loads environment variables from .env file
func LoadEnvFile(filepath string) error {
	// Check if .env file exists
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		log.Printf("No .env file found at %s, using system environment", filepath)
		return nil
	}

	// Load .env file
	err := godotenv.Load(filepath)
	if err != nil {
		return fmt.Errorf("failed to load .env file: %v", err)
	}

	log.Printf("Loaded environment variables from %s", filepath)
	return nil
}
// LoadDefaultEnvFile loads .env from current directory or parent directories
func LoadDefaultEnvFile() error {
	// Check current directory
	err := godotenv.Load()
	if err == nil {
		log.Printf("Loaded .env file from current directory")
		return nil
	}

	// Check parent directories
	dir, _ := os.Getwd()
	for i := 0; i < 3; i++ {
		if dir == "/" {
			break
		}
		envPath := filepath.Join(dir, ".env")
		if _, err := os.Stat(envPath); err == nil {
			err := godotenv.Load(envPath)
			if err == nil {
				log.Printf("Loaded .env file from %s", envPath)
				return nil
			}
		}
		dir = filepath.Dir(dir)
	}

	log.Printf("No .env file found in current or parent directories, using system environment")
	return nil
}