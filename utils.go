package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
	"golang.org/x/text/unicode/norm"
)

func readSrtFile(filepath string) ([]SrtSegment, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var results []SrtSegment
	scanner := bufio.NewScanner(file)
	var segment SrtSegment
	var textLines []string // Multiple lines of text in one segment

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" { // End of a segment
			segment.Text = strings.Join(textLines, "\n")
			results = append(results, segment)
			segment = SrtSegment{}
			textLines = nil
		} else if segment.ID == "" {
			segment.ID = line
		} else if segment.Time == "" {
			segment.Time = line
		} else { // Multiple lines of text
			textLines = append(textLines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	// Processing the last segment
	if segment.ID != "" {
		segment.Text = strings.Join(textLines, "\n")
		results = append(results, segment)
	}

	return results, nil
}

func saveSrtFile(translatedSegments []SrtSegment, originalSegments []SrtSegment, filePath string, bilingual bool) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Iterate through the segments and write them to the file.
	for i, segment := range translatedSegments {
		if bilingual && len(originalSegments) > i {
			file.WriteString(fmt.Sprintf("%s\n%s\n%s\n%s\n\n", segment.ID, segment.Time, originalSegments[i].Text, segment.Text))
		} else {
			file.WriteString(fmt.Sprintf("%s\n%s\n%s\n\n", segment.ID, segment.Time, segment.Text))
		}
	}

	return nil
}

func checkError(err error) {
	if err != nil {
		fmt.Println("Error :", err)
		os.Exit(1)
	}
}

func trimAnnotation(originalSegments, translatedSegments []SrtSegment) []SrtSegment {
	for i, translatedSegment := range translatedSegments {
		originalNewlines := strings.Count(originalSegments[i].Text, "\n")
		translatedNewlines := strings.Count(translatedSegment.Text, "\n")

		// If the number of newlines in the translated text is greater than in the original, find the position of the newline character after the number of newlines in the original text and truncate.

		if translatedNewlines > originalNewlines {
			parts := strings.SplitN(translatedSegment.Text, "\n", originalNewlines+2)
			translatedSegments[i].Text = strings.Join(parts[:originalNewlines+1], "\n")
		}
	}
	return translatedSegments
}

// takes a slice of SrtSegment and returns a new slice of SrtSegment
// with segments removed if their Text only contains repeated Unicode characters.
// It also adjusts the IDs of the remaining segments to be sequential starting from 1.
func deleteSrtSegmentsOnlyContainsRepeatedCharacters(segments []SrtSegment) []SrtSegment {
	results := make([]SrtSegment, 0)
	idCounter := 1

	for _, segment := range segments {
		if !isRepeatedChar(segment.Text) {
			segment.ID = fmt.Sprintf("%d", idCounter) // Update ID to be sequential
			results = append(results, segment)
			idCounter++
		}
	}

	return results
}

// isRepeatedChar checks if the string s consists of only repeated instances of a Unicode character.
func isRepeatedChar(s string) bool {
	if s == "" {
		return false
	}

	normalized := norm.NFKC.String(s)

	runeValue, size := utf8.DecodeRuneInString(normalized)
	if runeValue == utf8.RuneError {
		return false
	}

	for i := size; i < len(normalized); i += size {
		r, sSize := utf8.DecodeRuneInString(normalized[i:])
		if r != runeValue || sSize != size {
			return false
		}
	}

	return true
}

// This function reduces the number of repeated patterns (words) in the Text field
// of an SrtSegment that contain 2 to 6 characters and are repeated more than twice.
func reduceRepeatedPatterns(segments []SrtSegment) []SrtSegment {
	// Corrected regular expression: matches patterns of 2-6 non-whitespace characters that are repeated more than twice.
	// ([\S]{2,6}) Captures a group consisting of 2-6 non-whitespace characters.
	// \1{2,} Indicates that the group is repeated at least twice (occurs 3 times or more in total).
	regex := regexp2.MustCompile(`([\S]{2,6})(\1{2,})`, 0)

	for i, segment := range segments {
		normalizedText := norm.NFKC.String(segment.Text)
		for {
			m, _ := regex.FindStringMatch(normalizedText)
			if m == nil {
				break
			}

			// Extract the matched substring
			match := m.String()
			// Extract the repeated basic pattern (word)
			word := m.Groups()[1].String()
			// Only keep the pattern repeated twice
			newStr := word + word
			// Replace the matched part in the original string
			normalizedText = strings.Replace(normalizedText, match, newStr, 1)
		}
		// Update segment's Text field
		segments[i].Text = norm.NFKC.String(normalizedText)
	}

	return segments
}

// extendSegments adjusts the timing of SRT segments to ensure a minimum display duration, potentially extending beyond 1200ms.
// The function iterates through each segment, aiming for a minimum duration of 1200 milliseconds.
// If a segment's initial duration is shorter, its end time is extended based on the text length, up to a maximum of 3000ms.
// To prevent overlap, the end time is further adjusted if it would otherwise exceed the subsequent segment's start time.
func extendSegments(segments []SrtSegment) []SrtSegment {
	const timeLayout = "15:04:05,000"
	const minDuration = 1200 * time.Millisecond
	const maxDuration = 3000 * time.Millisecond
	const durationPerChar = 100 * time.Millisecond

	for i := 0; i < len(segments); i++ {
		times := strings.Split(segments[i].Time, " --> ")

		startTime, err := time.Parse(timeLayout, times[0])
		if err != nil {
			segments[i].Err = err
		}
		endTime, err := time.Parse(timeLayout, times[1])
		if err != nil {
			segments[i].Err = err
		}

		duration := endTime.Sub(startTime)
		if duration < minDuration {
			textLength := len(segments[i].Text)
			extendedDuration := time.Duration(textLength) * durationPerChar

			if extendedDuration < minDuration {
				extendedDuration = minDuration
			}
			if extendedDuration > maxDuration {
				extendedDuration = maxDuration
			}

			endTime = startTime.Add(extendedDuration)
			if i+1 < len(segments) {
				nextSegmentStartTime, _ := time.Parse(timeLayout, strings.Split(segments[i+1].Time, " --> ")[0])
				if endTime.After(nextSegmentStartTime) {
					endTime = nextSegmentStartTime.Add(-50 * time.Millisecond)
				}
			}
			segments[i].Time = fmt.Sprintf("%s --> %s", startTime.Format(timeLayout), endTime.Format(timeLayout))
		}
	}

	return segments
}

// replacePlaceholders replaces all occurrences of keys in the input string with their associated values.
func replacePlaceholders(input string, replacements map[string]string) string {
	for placeholder, value := range replacements {
		input = strings.ReplaceAll(input, placeholder, value)
	}
	return input
}
