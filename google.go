package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	translator "github.com/Conight/go-googletrans"
)

func translateViaGoogle(text string) (string, error) {
	c := translator.Config{
		Proxy: os.Getenv("http_proxy"), // Get the proxy from the environment variable
	}
	t := translator.New(c)
	result, err := t.Translate(text, "auto", "zh-cn") // Translate from auto-detected language to Simplified Chinese
	if err != nil {
		time.Sleep(30 * time.Second)
		result, err = t.Translate(text, "auto", "zh-cn") // retry once
		if err != nil {
			return "", err
		}
	}
	return result.Text, nil
}

func translateSrtSegmentsInBatches(segments []SrtSegment, config *Config) ([]SrtSegment, error) {
	const maxChars = 5000   // Google Translate web only accepts up to 5000 characters
	separator := "\n[ES]\n" // A separator for combining text
	results := make([]SrtSegment, len(segments))
	copy(results, segments)
	maxRequestsPerMinute := 3 // Default value for max requests per minute
	if config.maxRequestsPerMinute < 3 {
		maxRequestsPerMinute = config.maxRequestsPerMinute
	}

	errChan := make(chan error, 1)
	var wg sync.WaitGroup // WaitGroup is used to wait for all goroutines to complete.

	ticker := time.NewTicker(time.Minute / time.Duration(maxRequestsPerMinute)) // Create a ticker that resets every minute.
	defer ticker.Stop()
	concurrencyLimiter := make(chan struct{}, maxRequestsPerMinute) // Concurrency limiter to ensure we do not exceed maxRequestsPerMinute.
	for i := 0; i < maxRequestsPerMinute; i++ {
		concurrencyLimiter <- struct{}{} // Initialize the concurrency limiter
	}

	for startIndex := 0; startIndex < len(segments); {
		var combinedText string
		endIndex := startIndex

		// Combine text for each batch, ensuring we do not exceed the max character limit.
		for endIndex < len(segments) {
			nextText := combinedText
			if len(combinedText) != 0 {
				// Add a separator before appending only if we already have some text.
				nextText += separator
			}
			nextText += segments[endIndex].Text
			if len(nextText) > maxChars {
				break // This segment would exceed the limit, so it starts the next batch.
			}

			// If adding the segment does not exceed the limit, add it to the batch.
			combinedText = nextText
			endIndex++
		}

		if combinedText == "" {
			// This should not happen unless a single segment exceeds maxChars by itself.
			return nil, fmt.Errorf("single segment too large to process: %v characters", len(segments[startIndex].Text))
		}

		wg.Add(1)
		// Translate each segment in parallel.
		go func(startIndex, endIndex int, combinedText string) {
			defer wg.Done()
			<-concurrencyLimiter // Wait for permission from the concurrency limiter.
			<-ticker.C           // Allow a request to proceed whenever the ticker triggers.
			translatedCombinedText, err := translateViaGoogle(combinedText)

			// Handle errors
			if err != nil {
				select {
				case errChan <- err:
					//return
				default:
				}
			}

			// Split the translated text and assign it back to the segments.
			translatedTexts := strings.Split(translatedCombinedText, separator)
			for i, translatedText := range translatedTexts {
				results[startIndex+i].Text = translatedText
			}
			concurrencyLimiter <- struct{}{}
		}(startIndex, endIndex, combinedText)

		startIndex = endIndex // Set up for the next batch.
	}

	wg.Wait()
	close(errChan) // After all translation goroutines complete, close the error channel.

	// Check if there was any translation error.
	if err, ok := <-errChan; ok {
		//return nil, err
		fmt.Println("Error :", err)
	}
	return results, nil
}
