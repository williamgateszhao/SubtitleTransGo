package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
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
		os.Exit(0)
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

	runeValue, size := utf8.DecodeRuneInString(s)
	if runeValue == utf8.RuneError {
		return false
	}

	for i := size; i < len(s); i += size {
		r, sSize := utf8.DecodeRuneInString(s[i:])
		if r != runeValue || sSize != size {
			return false
		}
	}

	return true
}

// This function reduces the number of repeated patterns (words) in the Text field
// of an SrtSegment that contain 2 to 4 characters and are repeated more than twice.
func reduceRepeatedPatterns(segments []SrtSegment) []SrtSegment {
	regex := regexp2.MustCompile(`(([\S]{2,4})\2{3,})`, 0)
	for i, segment := range segments {
		for {
			m, _ := regex.FindStringMatch(segment.Text)
			if m == nil {
				break
			}

			// Extract the matched substring
			match := m.String()
			word := m.Groups()[2].String()

			// Build a new string that repeats only twice
			newStr := word + word

			// Replace the matched part in the original string
			segment.Text = strings.Replace(segment.Text, match, newStr, 1)
		}
		segments[i].Text = segment.Text
	}
	return segments
}
