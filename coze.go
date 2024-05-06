package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func translateViaCoze(originalText string, referenceTranslation string, config *Config) (string, error) {
	content := strings.Replace(config.userPrompt, "<ot>", originalText, 1)
	if referenceTranslation != "" {
		content = strings.Replace(content, "<rt>", referenceTranslation, 1)
	}

	// Build the request body.
	requestBody, err := json.Marshal(map[string]interface{}{
		"bot_id": config.botId,
		"user":   config.botId,
		"query":  content,
		"stream": false,
	})
	if err != nil {
		return "", err
	}

	// Establish a new HTTP request.
	req, err := http.NewRequest("POST", config.apiUrl, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", err
	}

	// Set the request header information.
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Authorization", "Bearer "+config.apiKey)
	req.Header.Set("Host", "api.coze.com")
	req.Header.Set("Connection", "keep-alive")

	// Execute the request.
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected HTTP status: %s", resp.Status)
	}

	// Read the response body.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Define a structure for receiving the coze API response.
	var response struct {
		Messages []struct {
			Role        string      `json:"role"`
			Type        string      `json:"type"`
			Content     string      `json:"content"`
			ContentType string      `json:"content_type"`
			ExtraInfo   interface{} `json:"extra_info"`
		} `json:"messages"`
		ConversationID string `json:"conversation_id"`
		Code           int    `json:"code"`
		Msg            string `json:"msg"`
	}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", err
	}

	// Check if the translation result was successfully received
	if len(response.Messages) == 0 || response.Messages[0].Content == "" {
		return "", fmt.Errorf("no translation content received from the API")
	}

	result := strings.TrimSpace(response.Messages[0].Content)
	return result, nil
}
