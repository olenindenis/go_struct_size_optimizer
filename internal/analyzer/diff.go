package analyzer

import (
	"fmt"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

const (
	reset   = "\033[0m"
	white   = "\033[97m"
	bold    = "\033[1m"
	bgRed   = "\033[1;41m"
	bgGreen = "\033[1;42m"
)

const diffWidth = 56

var dmp = diffmatchpatch.New()

func buildDiff(oldText, newText string) string {
	a, b, lineArray := dmp.DiffLinesToChars(oldText, newText)
	diffs := dmp.DiffCharsToLines(dmp.DiffMain(a, b, false), lineArray)

	divider := white + strings.Repeat("─", diffWidth) + reset

	var sb strings.Builder
	sb.WriteString(divider + "\n")

	lineNum := 1
	for _, d := range diffs {
		lines := strings.Split(d.Text, "\n")

		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		for _, line := range lines {
			switch d.Type {
			case diffmatchpatch.DiffEqual:
				fmt.Fprintf(&sb, "%s%2d   %s\n%s", white, lineNum, line, reset)
				lineNum++
			case diffmatchpatch.DiffDelete:
				fmt.Fprintf(&sb, "%s%s%s%2d   - %s\n%s", white, bgRed, bold, lineNum, line, reset)
				lineNum++
			case diffmatchpatch.DiffInsert:
				fmt.Fprintf(&sb, "%s%s%s     + %s\n%s", white, bold, bgGreen, line, reset)
			}
		}
	}

	sb.WriteString(divider + "\n")
	return sb.String()
}