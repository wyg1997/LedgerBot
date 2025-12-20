package feishu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/wyg1997/LedgerBot/config"
	"github.com/wyg1997/LedgerBot/internal/domain"
	"github.com/wyg1997/LedgerBot/pkg/logger"
)

// FeishuService handles Feishu API integration
type FeishuService struct {
	config     *config.FeishuConfig
	httpClient *http.Client
	token      string
	tokenExp   time.Time
	log        logger.Logger
}

// NewFeishuService creates a new Feishu service
func NewFeishuService(cfg *config.FeishuConfig) *FeishuService {
	return &FeishuService{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		log: logger.GetLogger(),
	}
}

// GetAccessToken gets access token for Feishu API
func (s *FeishuService) GetAccessToken() (string, error) {
	// Check if token is still valid
	if s.token != "" && time.Now().Before(s.tokenExp) {
		return s.token, nil
	}

	// Request new token
	url := "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"

	reqBody, _ := json.Marshal(map[string]string{
		"app_id":     s.config.AppID,
		"app_secret": s.config.AppSecret,
	})

	resp, err := s.httpClient.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code              int    `json:"code"`
		Message           string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}

	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	if result.Code != 0 {
		return "", fmt.Errorf("failed to get access token: %s", result.Message)
	}

	s.token = result.TenantAccessToken
	s.tokenExp = time.Now().Add(time.Duration(result.Expire-60) * time.Second) // Refresh 1 minute early

	s.log.Debug("Got new Feishu access token, expires in %d seconds", result.Expire)
	return s.token, nil
}

// SendMessage sends a message to a user
func (s *FeishuService) SendMessage(openID string, content string) error {
	token, err := s.GetAccessToken()
	if err != nil {
		return err
	}

	url := "https://open.feishu.cn/open-apis/message/v4/send"

	reqBody, _ := json.Marshal(map[string]interface{}{
		"open_id": openID,
		"msg_type": "text",
		"content": map[string]string{
			"text": content,
		},
	})

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"msg"`
	}

	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	if result.Code != 0 {
		return fmt.Errorf("failed to send message: %s", result.Message)
	}

	return nil
}

// AddRecordToBitable adds a record to Feishu bitable
func (s *FeishuService) AddRecordToBitable(appToken, tableToken string, fields map[string]interface{}) (string, error) {
	token, err := s.GetAccessToken()
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://open.feishu.cn/open-apis/bitable/v1/apps/%s/tables/%s/records", appToken, tableToken)

	reqBody, _ := json.Marshal(map[string]interface{}{
		"fields": fields,
	})

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to add record: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"msg"`
		Data    struct {
			Record struct {
				ID string `json:"record_id"`
			} `json:"record"`
		} `json:"data"`
	}

	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	if result.Code != 0 {
		return "", fmt.Errorf("failed to add record: %s", result.Message)
	}

	return result.Data.Record.ID, nil
}

// GetUserInfo gets user info by open ID
func (s *FeishuService) GetUserInfo(openID string) (*domain.User, error) {
	token, err := s.GetAccessToken()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://open.feishu.cn/open-apis/contact/v3/users/%s", openID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"msg"`
		Data    struct {
			User struct {
				Name   string `json:"name"`
				OpenID string `json:"open_id"`
			} `json:"user"`
		} `json:"data"`
	}

	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("failed to get user info: %s", result.Message)
	}

	return &domain.User{
		ID:         "feishu_" + result.Data.User.OpenID,
		Name:       result.Data.User.Name,
		PlatformID: result.Data.User.OpenID,
		Platform:   domain.PlatformFeishu,
	}, nil
}

// MessageCallback represents callback from Feishu
type MessageCallback struct {
	UUID   string `json:"uuid"`
	Token  string `json:"token"`
	Type   string `json:"type"`
	Event  struct {
		Type      string `json:"type"`
		AppID     string `json:"app_id"`
		TenantKey string `json:"tenant_key"`
		OpenID    string `json:"open_id"`
		OpenMessageID string `json:"open_message_id"`
		IsMention bool `json:"is_mention"`
		TextWithoutAtBot string `json:"text_without_at_bot"`
		Text      string `json:"text"`
	} `json:"event"`
}

// ProcessMessageCallback processes incoming message callback
func (s *FeishuService) ProcessMessageCallback(callback MessageCallback) (string, error) {
	userInfo, err := s.GetUserInfo(callback.Event.OpenID)
	if err != nil {
		return "", fmt.Errorf("failed to get user info: %v", err)
	}

	message := callback.Event.Text
	if callback.Event.IsMention && callback.Event.TextWithoutAtBot != "" {
		message = callback.Event.TextWithoutAtBot
	}

	s.log.Debug("Received message from %s: %s", userInfo.Name, message)

	// TODO: Process the message through bill use case
	// This is a basic response for now

	reply := fmt.Sprintf("收到您的记账信息：%s", message)
	if err := s.SendMessage(callback.Event.OpenID, reply); err != nil {
		s.log.Error("Failed to send reply: %v", err)
	}

	return "success", nil
}

// ListRecords gets records from bitable
func (s *FeishuService) ListRecords(appToken, tableToken string, pageSize, pageToken int) ([]map[string]interface{}, error) {
	token, err := s.GetAccessToken()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://open.feishu.cn/open-apis/bitable/v1/apps/%s/tables/%s/records?page_size=%d", appToken, tableToken, pageSize)
	if pageToken > 0 {
		url += fmt.Sprintf("&page_token=%d", pageToken)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list records: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"msg"`
		Data    struct {
			Items []map[string]interface{} `json:"items"`
			HasMore bool `json:"has_more"`
			PageToken int `json:"page_token"`
		} `json:"data"`
	}

	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("failed to list records: %s", result.Message)
	}

	return result.Data.Items, nil
}

// ListRecordsWithFilter gets records from bitable with filter conditions
func (s *FeishuService) ListRecordsWithFilter(appToken, tableToken string, filter map[string]interface{}) ([]map[string]interface{}, error) {
	token, err := s.GetAccessToken()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://open.feishu.cn/open-apis/bitable/v1/apps/%s/tables/%s/records/search", appToken, tableToken)

	reqBody, _ := json.Marshal(filter)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search records: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"msg"`
		Data    struct {
			Items []map[string]interface{} `json:"items"`
			HasMore bool `json:"has_more"`
			PageToken int `json:"page_token"`
		} `json:"data"`
	}

	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("failed to search records: %s", result.Message)
	}

	return result.Data.Items, nil
}