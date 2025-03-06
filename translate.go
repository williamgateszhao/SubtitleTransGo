package main

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func translateSrtSegmentsByLine(segments []SrtSegment, config *Config, referenceSegments ...[]SrtSegment) []SrtSegment {
	results := make([]SrtSegment, len(segments))
	copy(results, segments)

	var completedSegments int32
	// WaitGroup is used to wait for all goroutines to complete.
	var wg sync.WaitGroup

	// Create a timer that resets every minute.
	ticker := time.NewTicker(time.Minute / time.Duration(config.maxRequestsPerMinute))
	defer ticker.Stop()
	// Concurrency limiter to ensure the number of concurrent requests does not exceed maxRequestsPerMinute.
	concurrencyLimiter := make(chan struct{}, config.maxRequestsPerMinute)
	for i := 0; i < config.maxRequestsPerMinute; i++ {
		// Initialize the concurrency limiter
		concurrencyLimiter <- struct{}{}
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
			}

			results[i].Text = translatedText
			fmt.Println(segment.ID, "\n", segment.Time, "\n", segment.Text, "\n", translatedText)
			if err != nil {
				results[i].Err = err
				fmt.Println(err.Error())
			}
			progress := atomic.AddInt32(&completedSegments, 1)
			percentage := float32(progress) / float32(len(segments)) * 100
			fmt.Printf(" %0.2f%% completed\n", percentage)
			// Release the occupied concurrency limiter resource, allowing other waiting goroutines to continue execution.
			concurrencyLimiter <- struct{}{}
		}(i, segment)
	}

	wg.Wait()

	return results
}

func translateSrtSegmentsInBatches(segments []SrtSegment, config *Config) []SrtSegment {
	results := make([]SrtSegment, len(segments))
	copy(results, segments)

	//Assuming that a token has at least two characters,
	//and that input and output require double tokens,
	//the token that can be input is half of the maximum token,
	//so it is safe to take the maximum token for the maximum characters that can be entered
	maxChars := int(config.maxTokens)
	maxRequestsPerMinute := config.maxRequestsPerMinute
	if config.translator == "google" {
		maxChars = 5000 // Google Translate web only accepts up to 5000 characters
		maxRequestsPerMinute = 3
	}

	var completedSegments int32
	var wg sync.WaitGroup // WaitGroup is used to wait for all goroutines to complete.

	pattern := `(?ms)^(\d+)\n(\d{2}:\d{2}:\d{2},\d{3} --> \d{2}:\d{2}:\d{2},\d{3})\n(.*?)(?:\n\n|\z)`
	re := regexp.MustCompile(pattern)

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
			seg := segments[endIndex]
			block := seg.ID + "\n" + seg.Time + "\n" + seg.Text

			if len(combinedText) != 0 {
				combinedText += "\n\n" // Use double newline as separator
			}
			//Combine segments' texts ensuring the total does not exceed the character limit
			if len(combinedText)+len(block) > maxChars {
				break
			}
			combinedText += block
			endIndex++
		}

		if combinedText == "" {
			fmt.Println("single segment too large to process: ", len(segments[startIndex].Text), "characters")
			return nil
		}

		wg.Add(1)
		// Translate each segment in parallel.
		go func(startIndex, endIndex int, combinedText string) {
			defer wg.Done()
			<-concurrencyLimiter

			var translatedCombinedText string
			var err error

			// Add retry logic
			//If the translation result is deemed invalid (error marker present or translation count mismatch), retry up to a maximum number of attempts.
			maxRetries := 5
			retryCount := 1
			var translatedBlocks [][]string
			for {
				<-ticker.C // Wait for the ticker on each attempt

				switch config.translator {
				case "google":
					translatedCombinedText, err = translateViaGoogle(combinedText)
				case "openai":
					translatedCombinedText, err = translateViaOpenAI(combinedText, "", config)
				}

				// Check if it is an error response
				needRetry := false
				// Check if it starts with an error tag
				if strings.HasPrefix(translatedCombinedText, "[STGERROR]") {
					needRetry = true
				} else {
					translatedBlocks = re.FindAllStringSubmatch(translatedCombinedText, -1)
					if len(translatedBlocks) < (endIndex - startIndex) {
						needRetry = true
					} else {
						// Check if the length of each segment exceeds three times the original text
						for i := startIndex; i < endIndex && i-startIndex < len(translatedBlocks); i++ {
							translatedText := translatedBlocks[i-startIndex][3]
							originalText := segments[i].Text

							if len(translatedText) > len(originalText)*3 {
								fmt.Printf("Segment %d translation too long: %d chars vs %d chars (original). Retrying...\n", i, len(translatedText), len(originalText))
								needRetry = true
								break
							}
						}
					}
				}

				if needRetry && retryCount <= maxRetries {
					retryCount++
					fmt.Printf("Retrying batch (attempt %d/%d)...\n", retryCount, maxRetries)
					continue // Continue the loop, retry the batch
				}
				// Exit the loop if there is no error or the maximum number of retries has been reached
				break
			}

			if err != nil {
				for i := startIndex; i < endIndex; i++ {
					results[i].Err = err
				}
			}

			// Assign the translated lines back to the segments
			for i := startIndex; i < endIndex; i++ {
				if i-startIndex < len(translatedBlocks) {
					results[i].Text = translatedBlocks[i-startIndex][3]
				}
				fmt.Println(segments[i].ID, "\n", segments[i].Time, "\n", segments[i].Text, "\n", results[i].Text)
				if results[i].Err != nil {
					fmt.Println(results[i].Err.Error())
				}
				progress := atomic.AddInt32(&completedSegments, 1)
				percentage := float32(progress) / float32(len(segments)) * 100
				fmt.Printf(" %0.2f%% completed\n", percentage)
			}

			concurrencyLimiter <- struct{}{}
		}(startIndex, endIndex, combinedText)

		startIndex = endIndex // Set up for the next batch.
	}

	wg.Wait()

	return results
}
