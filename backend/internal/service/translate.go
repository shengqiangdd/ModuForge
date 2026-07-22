package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type TranslateService struct {
	client *http.Client
}

func NewTranslateService() *TranslateService {
	return &TranslateService{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Translate 翻译文本（使用 LibreTranslate 公共实例）
func (s *TranslateService) Translate(ctx context.Context, text, sourceLang, targetLang string) (string, error) {
	if text == "" || targetLang == "" {
		return "", fmt.Errorf("text and target language required")
	}
	if sourceLang == "" {
		sourceLang = "auto"
	}

	apiURL := "https://libretranslate.com/translate"
	body := fmt.Sprintf("q=%s&source=%s&target=%s&format=text",
		url.QueryEscape(text),
		url.QueryEscape(sourceLang),
		url.QueryEscape(targetLang),
	)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		// 公共 API 超时/失败时回退到简单标记
		return fmt.Sprintf("[%s] %s", targetLang, text), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("[%s] %s", targetLang, text), nil
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		TranslatedText string `json:"translatedText"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Sprintf("[%s] %s", targetLang, text), nil
	}

	return result.TranslatedText, nil
}

// TranslateModuleProps 翻译模块描述等多语言内容
func (s *TranslateService) TranslateModuleProps(ctx context.Context, props map[string]string, targetLang string) (map[string]string, error) {
	result := make(map[string]string, len(props))
	for key, val := range props {
		translated, err := s.Translate(ctx, val, "auto", targetLang)
		if err != nil {
			result[key] = val // fallback to original
		} else {
			result[key] = translated
		}
	}
	return result, nil
}
