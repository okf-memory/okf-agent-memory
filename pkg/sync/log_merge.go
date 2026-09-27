package sync

import (
	"bufio"
	"bytes"
	"fmt"
	"sort"
	"strings"
)

// MergeLogContent performs a 3-way/2-way semantic union of two OKF log.md files.
// It groups change entries by their "## YYYY-MM-DD" date headings, preserves descending
// chronological order, deduplicates identical bullet entries, and maintains any preamble.
func MergeLogContent(localBytes, remoteBytes []byte) ([]byte, error) {
	if len(localBytes) == 0 {
		return remoteBytes, nil
	}
	if len(remoteBytes) == 0 {
		return localBytes, nil
	}
	if bytes.Equal(localBytes, remoteBytes) {
		return localBytes, nil
	}

	localPreamble, localSections := parseLogSections(localBytes)
	remotePreamble, remoteSections := parseLogSections(remoteBytes)

	preamble := localPreamble
	if strings.TrimSpace(preamble) == "" {
		preamble = remotePreamble
	}
	if strings.TrimSpace(preamble) == "" {
		preamble = "# Knowledge Log\n"
	}

	// Collect unique dates
	dateMap := make(map[string][]string)
	for d, entries := range localSections {
		dateMap[d] = append(dateMap[d], entries...)
	}
	for d, entries := range remoteSections {
		dateMap[d] = append(dateMap[d], entries...)
	}

	var dates []string
	for d := range dateMap {
		dates = append(dates, d)
	}
	// Sort descending (latest dates first)
	sort.Slice(dates, func(i, j int) bool {
		return dates[i] > dates[j]
	})

	var sb strings.Builder
	sb.WriteString(strings.TrimRight(preamble, "\r\n"))
	sb.WriteString("\n\n")

	for i, d := range dates {
		rawEntries := dateMap[d]
		// Deduplicate entries while preserving first-seen order
		seen := make(map[string]struct{})
		var uniqueEntries []string
		for _, e := range rawEntries {
			trimmed := strings.TrimSpace(e)
			if trimmed == "" {
				continue
			}
			if _, ok := seen[trimmed]; !ok {
				seen[trimmed] = struct{}{}
				uniqueEntries = append(uniqueEntries, trimmed)
			}
		}

		fmt.Fprintf(&sb, "## %s\n", d)
		for _, e := range uniqueEntries {
			fmt.Fprintf(&sb, "%s\n", e)
		}
		if i < len(dates)-1 {
			sb.WriteString("\n")
		}
	}

	return []byte(sb.String()), nil
}

func parseLogSections(data []byte) (string, map[string][]string) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var preambleLines []string
	sections := make(map[string][]string)

	currentDate := ""
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "## ") {
			datePart := strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			currentDate = datePart
			continue
		}

		if currentDate == "" {
			preambleLines = append(preambleLines, line)
		} else {
			if trimmed != "" {
				sections[currentDate] = append(sections[currentDate], line)
			}
		}
	}

	return strings.Join(preambleLines, "\n"), sections
}
