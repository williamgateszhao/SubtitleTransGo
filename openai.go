package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
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

func translateSrtSegmentsByLine(segments []SrtSegment, config *Config, referenceSegments ...[]SrtSegment) ([]SrtSegment, error) {
	results := make([]SrtSegment, len(segments))
	errChan := make(chan error, 1) // Error channel, used to receive errors from multiple goroutines.
	var wg sync.WaitGroup          // WaitGroup is used to wait for all goroutines to complete.

	ticker := time.NewTicker(time.Minute / time.Duration(config.maxRequestsPerMinute)) // Create a timer that resets every minute.
	defer ticker.Stop()
	concurrencyLimiter := make(chan struct{}, config.maxRequestsPerMinute) // Concurrency limiter to ensure the number of concurrent requests does not exceed maxRequestsPerMinute.

	for i := 0; i < config.maxRequestsPerMinute; i++ {
		concurrencyLimiter <- struct{}{} // Initialize the concurrency limiter
	}

	for i, segment := range segments {
		wg.Add(1)
		// Concurrently translate each segment.
		go func(i int, segment SrtSegment) {
			defer wg.Done()
			<-concurrencyLimiter // Wait for permission from the concurrency limiter.
			<-ticker.C           // Allow a request to proceed whenever the ticker triggers.

			var translatedText string
			var err error
			var referenceTranslation string
			if referenceSegments != nil {
				referenceTranslation = referenceSegments[0][i].Text
			}
			switch config.translator {
			case "openai":
				translatedText, err = translateViaOpenAI(segment.Text, referenceTranslation, config)
			case "coze":
				translatedText, err = translateViaCoze(segment.Text, referenceTranslation, config)
			}

			// If an error occurs, send it to the error channel, but only receive the first error.
			if err != nil {
				if referenceSegments != nil {
					translatedText = referenceSegments[0][i].Text
				}

				select {
				case errChan <- err: // Send the error and exit.
					//return
				default: // If the error channel is full (an error has already been sent), ignore subsequent errors.
				}

			}

			results[i] = SrtSegment{
				ID:   segment.ID,
				Time: segment.Time,
				Text: translatedText,
			}

			fmt.Println(segment.ID, "\n", segment.Time, "\n", segment.Text, "\n", translatedText)

			concurrencyLimiter <- struct{}{} // Release the occupied concurrency limiter resource, allowing other waiting goroutines to continue execution.

		}(i, segment)
	}

	wg.Wait()
	close(errChan) // Close the error channel after all the translation goroutines are done.

	// Check if there was a translation error.
	if err, ok := <-errChan; ok {
		//return nil, err
		fmt.Println("Error :", err)
	}
	return results, nil
}
