package main

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// translateSrtSegmentsInBatches processes and translates SRT segments in batches with rate limiting
// and concurrency control. It combines segments up to the maximum token limit, translates them
// using the configured translator, and handles retries for failed translations. If singleLine mode
// is enabled, failed batch translations will be retried individually. Returns the translated segments
// with the same structure as the input, preserving original metadata.
func translateSrtSegmentsInBatches(segments []SrtSegment, referenceSegments []SrtSegment, config *Config) []SrtSegment {
	results := make([]SrtSegment, len(segments))
	copy(results, segments) // Pre-populate with original data to simplify later assignments

	var completedSegments int32
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Setup rate limiting
	maxRequestsPerMinute := config.maxRequestsPerMinute
	ticker := time.NewTicker(time.Minute / time.Duration(maxRequestsPerMinute))
	defer ticker.Stop()

	// Limit concurrent requests
	concurrencyLimiter := make(chan struct{}, maxRequestsPerMinute)
	defer close(concurrencyLimiter)

	for startIndex := 0; startIndex < len(segments); {
		combinedText, combinedReference, endIndex := combineText(segments, referenceSegments, startIndex, int(config.maxTokens))

		if combinedText == "" {
			fmt.Println("single segment too large to process: ", len(segments[startIndex].Text), "characters")
			return nil
		}

		wg.Add(1)
		concurrencyLimiter <- struct{}{} // Acquire semaphore

		// Process batch in a goroutine
		go func(startIndex, endIndex int, combinedText, combinedReference string) {
			defer wg.Done()
			defer func() { <-concurrencyLimiter }() // Release semaphore

			translatedSegments, err := translateSegments(startIndex, endIndex, combinedText, combinedReference, config, ticker)

			if err != nil {
				// Batch failed: Retry each segment individually
				if config.singleLine {
					for i := startIndex; i < endIndex; i++ {
						concurrencyLimiter <- struct{}{}

						fmt.Printf("Retrying ID %s in single line mode\n", segments[i].ID)
						translatedSingleLine, err := translateSegments(i, i+1, formatSegment(segments[i]), "", config, ticker)

						mu.Lock()
						if err != nil {
							if results[i].Err == nil {
								results[i].Err = err
							}
						} else {
							results[i].Text = translatedSingleLine[0].Text
						}
						printProgress(segments[i], results[i], len(segments), &completedSegments)
						mu.Unlock()

						<-concurrencyLimiter
					}
				} else {
					mu.Lock()
					// If not in single line mode, mark all segments as failed
					for i := startIndex; i < endIndex; i++ {
						results[i].Err = err
						printProgress(segments[i], results[i], len(segments), &completedSegments)
					}
					mu.Unlock()
				}
			} else { // Batch succeeded
				mu.Lock()
				for i := startIndex; i < endIndex; i++ {
					idx := i - startIndex
					if idx < len(translatedSegments) {
						results[i].Text = translatedSegments[idx].Text
					} else if results[i].Err == nil {
						results[i].Err = fmt.Errorf("no translation available for this segment")
					}
					printProgress(segments[i], results[i], len(segments), &completedSegments)
				}
				mu.Unlock()
			}
		}(startIndex, endIndex, combinedText, combinedReference)

		startIndex = endIndex // Move to next batch
	}

	wg.Wait()
	return results
}

func combineText(segments []SrtSegment, referenceSegments []SrtSegment, startIndex int, maxChars int) (string, string, int) {
	var combinedText, combinedReference string
	endIndex := startIndex

	// Combine segments until reaching character limit
	for endIndex < len(segments) {
		// Format current segment as a block
		block := formatSegment(segments[endIndex])

		// Format corresponding reference segment if available
		blockReference := ""
		if endIndex < len(referenceSegments) {
			blockReference = formatSegment(referenceSegments[endIndex])
		}

		// Check if adding these blocks would exceed the character limit
		separator := ""
		referenceSeparator := ""
		if len(combinedText) > 0 {
			separator = "\n\n" // Use double newline as separator
		}
		if len(combinedReference) > 0 {
			referenceSeparator = "\n\n"
		}

		totalLength := len(combinedText) + len(separator) + len(block) +
			len(combinedReference) + len(referenceSeparator) + len(blockReference)

		if totalLength > maxChars {
			break
		}

		// Add separator and block
		combinedText += separator + block
		combinedReference += referenceSeparator + blockReference
		endIndex++
	}

	return combinedText, combinedReference, endIndex
}

func translateSegments(startIndex int, endIndex int, combinedText string, combinedReference string, config *Config, ticker *time.Ticker) ([]SrtSegment, error) {
	for retryCount := 1; retryCount <= config.maxRetries; retryCount++ {
		<-ticker.C // Wait for the ticker on each attempt

		// Perform translation based on configured translator
		translatedText, err := config.TranslatorImpl.translate(combinedText, combinedReference, config)

		// Check for translation issues
		needRetry, translatedBlocks, retryReason := checkTranslationResult(translatedText, err, startIndex, endIndex)

		if !needRetry {
			// Translation successful, extract segments
			translatedSegments := make([]SrtSegment, 0, len(translatedBlocks))
			for _, match := range translatedBlocks {
				translatedSegments = append(translatedSegments, SrtSegment{
					Text: match[3],
				})
			}
			return translatedSegments, nil
		}

		// Handle retry logic
		if retryCount < config.maxRetries {
			fmt.Printf("Retrying batch (attempt %d/%d): %s\n", retryCount, config.maxRetries, retryReason)
			// Add exponential backoff for retries
			time.Sleep(time.Duration(retryCount) * time.Second)
		}
	}

	return nil, fmt.Errorf("failed to translate segments after %d attempts", config.maxRetries)
}

// checkTranslationResult validates the translation output
func checkTranslationResult(translatedText string, err error, startIndex, endIndex int) (bool, [][]string, string) {
	pattern := `(?m)^(\d+)\n(\d{2}.*?\d{2}.*?\d{2}.*?-+>.*?\d{2}.*?\d{2}.*?\d{3})\n(.*?)(?:\n\n|\z)`
	re := regexp.MustCompile(pattern)
	translatedText = strings.Replace(translatedText, "\u200b", "", -1)
	// Check for error response
	if err != nil {
		return true, nil, fmt.Sprintf("Translation error: %v", err)
	}

	// Check if it starts with an error tag
	if strings.HasPrefix(translatedText, "[STGERROR]") {
		return true, nil, fmt.Sprintf("Error response received: %s", translatedText)
	}

	// Check segment count
	translatedBlocks := re.FindAllStringSubmatch(translatedText, -1)
	expectedCount := endIndex - startIndex
	if len(translatedBlocks) != expectedCount {
		return true, nil, fmt.Sprintf("Expected %d translated segments but got %d",
			expectedCount, len(translatedBlocks))
	}

	// Translation is valid
	return false, translatedBlocks, ""
}

func printProgress(segment, result SrtSegment, len int, completedSegments *int32) {
	fmt.Printf("%s\n%s\n%s\n%s\n",
		segment.ID,
		segment.Time,
		segment.Text,
		result.Text)

	progress := atomic.AddInt32(completedSegments, 1)
	percentage := float32(progress) / float32(len) * 100
	fmt.Printf(" %.2f%% completed\n", percentage)
}

// Helper function to format a segment as a block
func formatSegment(segment SrtSegment) string {
	return segment.ID + "\n" + segment.Time + "\n" + segment.Text
}
