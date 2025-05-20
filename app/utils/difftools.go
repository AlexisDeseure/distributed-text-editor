package utils

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// A Diff object describes a modification that was applied to a text
type Diff struct {
	Pos       int    // Start index
	NbDeleted int    // Number of characters to delete (0 if text was only added)
	NewText   string // Text to insert at the position ("" if text was only deleted)
}

// This method compares oldText and newText and returns a Diff slice (array), with one Diff per change block
func ComputeDiffs(oldText, newText string) []Diff {
	// A rune is an integer type that represents a character
	rOld := []rune(oldText)
	rNew := []rune(newText)

	// Get the length of each text
	nOld, nNew := len(rOld), len(rNew)

	// LCS matrix (longest common subsequence)
	// Size = (nOld + 1) x (nNew + 1)
	// dp[i][j] will contain the length of the LCS between rOld[i:] and rNew[j:]
	dp := make([][]int, nOld+1)

	for i := range dp {
		// Each line dp[i] is a slice of size nNew + 1, set to 0 first
		dp[i] = make([]int, nNew+1)
	}

	// Go through the matrix, from bottom right to top left
	for i := nOld - 1; i >= 0; i-- {
		for j := nNew - 1; j >= 0; j-- {
			// Compare the character (rune) at position i of the old text
			// and the character at position j of the new text
			if rOld[i] == rNew[j] {
				// If they are the same character, we increase the length of the LCS by one
				// compared to dp[i+1][j+1], which represents the length of the LCS for next suffixes
				dp[i][j] = dp[i+1][j+1] + 1
			} else {
				// Else, we take the max between:
				// - dp[i + 1][j] (ignore rOld[i] and compare rOld[i + 1:] with rNew[j:])
				// - dp[i][j + 1] (ignore rNew[j] and compare rOld[i] with rNew[j + 1:])
				// This way, we choose one character or the other depending on which one will create the longest LCS
				dp[i][j] = max(dp[i+1][j], dp[i][j+1])
			}
		}
	}

	// Extract the Diffs
	diffs := []Diff{}
	i, j, pos := 0, 0, 0

	for i < nOld || j < nNew {
		// If runes match, we continue
		if i < nOld && j < nNew && rOld[i] == rNew[j] {

			i++
			j++
			pos++

			// If they don't match, it's a new Diff
		} else {

			// Start position of a new Diff block
			start := pos

			oldLen := 0

			// Each rune that is in oldText but not in newText must be marked as deleted (oldLen++)
			for i < nOld && (j >= nNew || dp[i+1][j] >= dp[i][j+1]) {

				i++
				oldLen++
				pos++
			}

			insRunes := []rune{}

			// Then, each rune that is in newText but not in oldText must be added to the insertion String (insRunes)
			for j < nNew && (i >= nOld || dp[i][j+1] > dp[i+1][j]) {

				insRunes = append(insRunes, rNew[j])
				j++
			}

			// Create a new Diff object and add it to the slice (array)
			diffs = append(diffs, Diff{
				Pos:       start,
				NbDeleted: oldLen,
				NewText:   string(insRunes),
			})
		}
	}

	// Return the slice
	return diffs
}

// Apply diffs to a base string, with the correct order
func ApplyDiffsSequential(base string, diffs []Diff) string {
	rBase := []rune(base)

	for _, d := range diffs {
		// clamp pos
		pos := d.Pos
		if pos < 0 {
			pos = 0
		}
		if pos > len(rBase) {
			pos = len(rBase)
		}

		// clamp oldLen
		oldLen := d.NbDeleted
		if oldLen < 0 {
			oldLen = 0
		}
		if pos+oldLen > len(rBase) {
			oldLen = len(rBase) - pos
		}

		before := rBase[:pos]
		after := rBase[pos+oldLen:]
		ins := []rune(d.NewText)

		// The diff is applied
		rBase = append(before, append(ins, after...)...)
	}

	return string(rBase)
}

// This method applies one or multiple Diffs to a base text, and returns the resulting text
// Diffs are applied from last to first so that the indices stay valid after each Diff is applied

func ApplyDiffs(base string, diffs []Diff) string {
	// Convert the base text to a slice of runes
	rBase := []rune(base)

	// Iterate backwards through the slice
	for i := len(diffs) - 1; i >= 0; i-- {
		d := diffs[i]

		// If d.Pos is beyond the current text, pad with spaces
		if d.Pos > len(rBase) {
			pad := make([]rune, d.Pos-len(rBase))
			for j := range pad {
				pad[j] = ' '
			}
			rBase = append(rBase, pad...)
		}

		// Compute safe start and end for deletion
		start := d.Pos
		end := d.Pos + d.NbDeleted
		if start < 0 {
			start = 0
		}
		if end > len(rBase) {
			end = len(rBase)
		}

		// Split before/after the deletion window
		before := rBase[:start]
		after := rBase[end:]

		// Insert the new text
		ins := []rune(d.NewText)
		rBase = append(before, append(ins, after...)...)
	}

	return string(rBase)
}

// Convert a diff object to a json string
func (d Diff) String() string {
	b, _ := json.Marshal(d)
	return string(b)
}

// Get a list of diff objects based on two version of a text, and then save those diffs to the file
func SaveModifs(oldText, newText string, saveFilePath string) error {
	for _, d := range ComputeDiffs(oldText, newText) {
		if err := appendDiffToFile(d, saveFilePath); err != nil {
			return err
		}
	}
	return nil
}

// Convert a diff object to a string, and save it to the file
func appendDiffToFile(d Diff, saveFilePath string) error {
	f, err := os.OpenFile(saveFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.WriteString(d.String() + "\n"); err != nil {
		return err
	}
	return nil
}

// Read the save file starting from a given line, and get a list of diff objects
func readDiffsFromFile(startLine int, saveFilePath string) ([]Diff, error) {
	initialize(saveFilePath)
	f, err := os.Open(saveFilePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var diffs []Diff
	scanner := bufio.NewScanner(f)
	line := -1
	for scanner.Scan() {
		line++
		if line < startLine {
			continue
		}
		var d Diff
		if err := json.Unmarshal(scanner.Bytes(), &d); err != nil {
			return nil, fmt.Errorf("ligne %d: %w", line, err)
		}
		diffs = append(diffs, d)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return diffs, nil
}

func GetUpdatedTextFromFile(startLine int, baseText string, saveFilePath string) (string, error) {
	diffs, err := readDiffsFromFile(startLine, saveFilePath)
	if err != nil {
		return baseText, err
	}
	return ApplyDiffsSequential(baseText, diffs), nil
}

// Get the number of lines (diff objects) that were written after a given line
func LineCountSince(startLine int, saveFilePath string) int {
	initialize(saveFilePath)
	f, err := os.Open(saveFilePath)
	if err != nil {
		// If the file cannot be opened, the method considers that there is no new line
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum, count := 0, 0
	for scanner.Scan() {
		lineNum++
		if lineNum >= startLine {
			count++
		}
	}
	return count
}

func initialize(saveFilePath string) {
	// Make sure that the log file exists before using it
	if _, err := os.Stat(saveFilePath); os.IsNotExist(err) {
		// Create necessary directories if they don't exist
		if err := os.MkdirAll(filepath.Dir(saveFilePath), os.ModePerm); err != nil {
			log.Fatalf("Couldn't create directories for %s : %v", saveFilePath, err)
		}
		// Create an empty file
		if f, err := os.Create(saveFilePath); err != nil {
			log.Fatalf("Couldn't create %s : %v", saveFilePath, err)
		} else {
			f.Close()
		}
	}
}
