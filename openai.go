package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func translateViaOpenAI(originalText string, referenceTranslation string, config *Config) (string, error) {
	content := strings.Replace(config.userPrompt, "<ot>", originalText, 1)
	if referenceTranslation != "" {
		content = strings.Replace(content, "<rt>", referenceTranslation, 1)
	}

	// Create an array of messages required for the API request.
	messages := []map[string]string{
		{
			"role":    "system",
			"content": config.systemPrompt,
		},
		{
			"role":    "user",
			"content": content,
		},
	}

	// Build the request body.
	requestBody, err := json.Marshal(map[string]interface{}{
		"model":       config.modelName,
		"temperature": config.temperature,
		"top_p":       config.topP,
		"max_tokens":  config.maxTokens,
		"messages":    messages,
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
	req.Header.Set("Authorization", "Bearer "+config.apiKey)

	// Execute the request.
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected HTTP status: %s", resp.Status)
	}
	defer resp.Body.Close()

	// Read the response body.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Define a structure for receiving the API response.
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", err
	}

	// Check if the translation result was successfully received
	if len(response.Choices) == 0 || response.Choices[0].Message.Content == "" {
		//return "", fmt.Errorf("no translation content received from the API")
		return "[STGERROR]" + originalText, nil
	}

	result := strings.TrimSpace(response.Choices[0].Message.Content)
	return result, nil

}
