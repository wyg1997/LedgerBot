package config

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	// Server configuration
	Server ServerConfig

	// Platform configurations
	Feishu FeishuConfig

	// AI configuration
	AI AIConfig

	// Storage configuration
	Storage StorageConfig

	// Cache configuration
	Cache CacheConfig
}

type ServerConfig struct {
	Port         string
	ReadTimeout  int    // seconds
	WriteTimeout int    // seconds
}

type FeishuConfig struct {
	AppID        string
	AppSecret    string
	BitableURL   string // 多维表格URL，格式：https://example.feishu.cn/base/APP_TOKEN?table=TABLE_TOKEN
	EncryptKey   string // 可选的加密密钥
	Verification string // 可选的验证 token
	// 多维表格字段名配置
	FieldDescription string // 描述字段名
	FieldAmount      string // 金额字段名
	FieldType        string // 类型字段名(Income/Expense)
	FieldCategory    string // 分类字段名
	FieldDate        string // 日期字段名
	FieldUserName    string // 用户名字段名
	FieldOriginalMsg string // 原始消息字段名
}


type AIConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

type StorageConfig struct {
	UserMappingFile string // 用户映射文件路径
	DataDir         string // 数据存储目录
	LogLevel        string // 日志级别
}

type CacheConfig struct {
	TTL          int  // 缓存过期时间（秒）
	CleanUpIntvl int  // 清理间隔（秒）
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	// Try to load .env file before reading config
	err := LoadDefaultEnvFile()
	if err != nil {
		log.Printf("Failed to load .env file: %v", err)
	}

	return &Config{
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8080"),
			ReadTimeout:  getEnvAsInt("SERVER_READ_TIMEOUT", 30),
			WriteTimeout: getEnvAsInt("SERVER_WRITE_TIMEOUT", 30),
		},
		Feishu: FeishuConfig{
			AppID:            getEnv("FEISHU_APP_ID", ""),
			AppSecret:        getEnv("FEISHU_APP_SECRET", ""),
			BitableURL:       getEnv("FEISHU_BITABLE_URL", ""),
			EncryptKey:       getEnv("FEISHU_ENCRYPT_KEY", ""),
			Verification:     getEnv("FEISHU_VERIFICATION_TOKEN", ""),
			FieldDescription: getEnv("FEISHU_FIELD_DESCRIPTION", "描述"),
			FieldAmount:      getEnv("FEISHU_FIELD_AMOUNT", "金额"),
			FieldType:        getEnv("FEISHU_FIELD_TYPE", "类型"),
			FieldCategory:    getEnv("FEISHU_FIELD_CATEGORY", "分类"),
			FieldDate:        getEnv("FEISHU_FIELD_DATE", "日期"),
			FieldUserName:    getEnv("FEISHU_FIELD_USER_NAME", "用户"),
			FieldOriginalMsg: getEnv("FEISHU_FIELD_ORIGINAL_MSG", "原始消息"),
		},
		AI: AIConfig{
			BaseURL: getEnv("AI_BASE_URL", "https://api.openai.com"),
			APIKey:  getEnv("AI_API_KEY", ""),
			Model:   getEnv("AI_MODEL", "gpt-3.5-turbo"),
		},
		Storage: StorageConfig{
			UserMappingFile: getEnv("USER_MAPPING_FILE", "./data/user_mapping.json"),
			DataDir:         getEnv("DATA_DIR", "./data"),
			LogLevel:        getEnv("LOG_LEVEL", "info"),
		},
		Cache: CacheConfig{
			TTL:          getEnvAsInt("CACHE_TTL", 3600),    // 1 hour
			CleanUpIntvl: getEnvAsInt("CACHE_CLEANUP", 300), // 5 minutes
		},
	}
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt gets an environment variable as an integer
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}

// getEnvAsFloat gets an environment variable as a float
func getEnvAsFloat(key string, defaultValue float64) float64 {
	valueStr := getEnv(key, "")
	if value, err := strconv.ParseFloat(valueStr, 64); err == nil {
		return value
	}
	return defaultValue
}

// getEnvAsBool gets an environment variable as a boolean
func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := getEnv(key, "")
	if value, err := strconv.ParseBool(valueStr); err == nil {
		return value
	}
	return defaultValue
}

// IsValid checks if the configuration is valid
func (c *Config) IsValid() error {
	if c.Feishu.AppID == "" || c.Feishu.AppSecret == "" {
		return &ConfigError{Field: "feishu", Message: "Feishu AppID and AppSecret are required"}
	}
	if c.AI.APIKey == "" {
		return &ConfigError{Field: "ai", Message: "AI API key is required"}
	}
	return nil
}

// ConfigError represents a configuration error
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return e.Field + ": " + e.Message
}