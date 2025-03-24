package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	googletrans "github.com/Conight/go-googletrans"
)

type Translator interface {
	translate(originalText string, referenceTranslation string, config *Config) (string, error)
}

type GoogleTranslator struct {
}

type OpenAITranslator struct {
}

// OpenAIResponse represents the structure of the API response
// OpenAIResponse represents the structure of the response from OpenAI API
type OpenAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// translate sends a request to OpenAI API to translate text
// It handles both simple translation and translation with reference
func (o *OpenAITranslator) translate(originalText string, referenceTranslation string, config *Config) (string, error) {
	// Prepare content based on whether reference translation is provided
	content := strings.Replace(config.userPrompt, "<ot>", originalText, 1)
	if referenceTranslation != "" {
		content = strings.Replace(config.userPrompt3, "<ot>", originalText, 1)
		content = strings.Replace(content, "<rt>", referenceTranslation, 1)
	}

	// Prepare request payload
	payload := map[string]interface{}{
		"model":       config.modelName,
		"temperature": config.temperature,
		"top_p":       config.topP,
		"max_tokens":  config.maxTokens,
		"messages": []map[string]string{
			{"role": "system", "content": config.systemPrompt},
			{"role": "user", "content": content},
		},
	}

	requestBody, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create and execute HTTP request
	req, err := http.NewRequest(http.MethodPost, config.apiUrl, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected HTTP status: %s", resp.Status)
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var response OpenAIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Handle empty response
	if len(response.Choices) == 0 || response.Choices[0].Message.Content == "" {
		return "[STGERROR]" + originalText, nil
	}

	return strings.TrimSpace(response.Choices[0].Message.Content), nil
}

func (g *GoogleTranslator) translate(originalText string, referenceTranslation string, config *Config) (string, error) {
	// Create Google Translate client with proxy from environment
	t := googletrans.New(googletrans.Config{
		Proxy: os.Getenv("http_proxy"),
	})

	result, err := t.Translate(originalText, "auto", config.targetLang)
	if err != nil {
		return "", err
	}

	return result.Text, nil
}
