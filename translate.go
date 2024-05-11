package main

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func translateSrtSegmentsByLine(segments []SrtSegment, config *Config, referenceSegments ...[]SrtSegment) []SrtSegment {
	results := make([]SrtSegment, len(segments))
	copy(results, segments)

	var completedSegments int32
	var wg sync.WaitGroup // WaitGroup is used to wait for all goroutines to complete.

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

			results[i].Text = translatedText
			fmt.Println(segment.ID, "\n", segment.Time, "\n", segment.Text, "\n", translatedText)
			if err != nil {
				results[i].Err = err
				fmt.Println(err.Error())
			}
			progress := atomic.AddInt32(&completedSegments, 1)
			percentage := float32(progress) / float32(len(segments)) * 100
			fmt.Printf(" %0.2f%% completed\n", percentage)

			concurrencyLimiter <- struct{}{} // Release the occupied concurrency limiter resource, allowing other waiting goroutines to continue execution.
		}(i, segment)
	}

	wg.Wait()

	return results
}

func translateSrtSegmentsInBatches(segments []SrtSegment, config *Config) []SrtSegment {
	results := make([]SrtSegment, len(segments))
	copy(results, segments)

	maxChars := int(config.maxTokens / 2)
	maxRequestsPerMinute := config.maxRequestsPerMinute
	if config.translator == "google" {
		maxChars = 5000 // Google Translate web only accepts up to 5000 characters
		maxRequestsPerMinute = 3
	}

	var completedSegments int32
	var wg sync.WaitGroup // WaitGroup is used to wait for all goroutines to complete.

	ticker := time.NewTicker(time.Minute / time.Duration(maxRequestsPerMinute)) // Create a ticker that resets every minute.
	defer ticker.Stop()
	concurrencyLimiter := make(chan struct{}, maxRequestsPerMinute) // Concurrency limiter to ensure we do not exceed maxRequestsPerMinute.
	for i := 0; i < maxRequestsPerMinute; i++ {
		concurrencyLimiter <- struct{}{} // Initialize the concurrency limiter
	}

	for startIndex := 0; startIndex < len(segments); {
		var combinedText string
		var lineCounts []int // Store line counts for each segment
		endIndex := startIndex

		// Combine text for each batch, ensuring we do not exceed the max character limit.
		for endIndex < len(segments) && (len(combinedText)+len(segments[endIndex].Text)+(len(lineCounts)-1)) < maxChars {
			if len(combinedText) != 0 {
				combinedText += "\n" // Use newline as separator
			}
			combinedText += segments[endIndex].Text
			lineCounts = append(lineCounts, strings.Count(segments[endIndex].Text, "\n")+1) // Record line count, +1 for the separation
			endIndex++
		}

		if combinedText == "" {
			fmt.Println("single segment too large to process: ", len(segments[startIndex].Text), "characters")
			return nil
		}

		wg.Add(1)
		// Translate each segment in parallel.
		go func(startIndex, endIndex int, combinedText string, lineCounts []int) {
			defer wg.Done()
			<-concurrencyLimiter
			<-ticker.C
			translatedCombinedText, err := translateViaGoogle(combinedText)
			if err != nil {
				for i := startIndex; i < endIndex; i++ {
					results[i].Err = err
				}
			}

			// Split the translated text by newline characters using strings.Split
			translatedLines := strings.Split(translatedCombinedText, "\n")

			// Determine the lines for each paragraph through lineCounts
			currentLine := 0
			for i, numLines := range lineCounts {
				// Concatenate the required number of lines (numLines) for the current paragraph
				if currentLine+numLines > len(translatedLines) {
					numLines = len(translatedLines) - currentLine
				}
				segmentLines := translatedLines[currentLine : currentLine+numLines]
				results[startIndex+i].Text = strings.Join(segmentLines, "\n")

				// Update the pointer for the current line
				currentLine += numLines

				fmt.Println(segments[startIndex+i].ID, "\n", segments[startIndex+i].Time, "\n", segments[startIndex+i].Text, "\n", results[startIndex+i].Text)
				if err != nil {
					results[i].Err = err
					fmt.Println(err.Error())
				}
				progress := atomic.AddInt32(&completedSegments, 1)
				percentage := float32(progress) / float32(len(segments)) * 100
				fmt.Printf(" %0.2f%% completed\n", percentage)
			}

			concurrencyLimiter <- struct{}{}
		}(startIndex, endIndex, combinedText, lineCounts)

		startIndex = endIndex // Set up for the next batch.
	}

	wg.Wait()

	return results
}
