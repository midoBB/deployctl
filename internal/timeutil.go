package internal

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func ParseSystemdUSec(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "0" || strings.EqualFold(trimmed, "n/a") {
		return time.Time{}, fmt.Errorf("timestamp not available")
	}

	microseconds, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse microsecond timestamp %q: %w", trimmed, err)
	}
	if microseconds <= 0 {
		return time.Time{}, fmt.Errorf("timestamp must be greater than zero")
	}

	return time.UnixMicro(microseconds), nil
}

func ParseSystemdTimestamp(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.EqualFold(trimmed, "n/a") {
		return time.Time{}, fmt.Errorf("timestamp not available")
	}

	layouts := []string{
		time.RFC3339,
		"Mon 2006-01-02 15:04:05 MST",
		"Mon 2006-01-02 15:04:05 -0700",
		"Mon 2006-01-02 15:04:05",
	}

	for _, layout := range layouts {
		parsed, err := time.Parse(layout, trimmed)
		if err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, fmt.Errorf("unrecognized systemd timestamp %q", trimmed)
}

func FormatUptime(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	duration = duration.Round(time.Second)

	days := duration / (24 * time.Hour)
	duration -= days * 24 * time.Hour

	hours := duration / time.Hour
	duration -= hours * time.Hour

	minutes := duration / time.Minute
	duration -= minutes * time.Minute

	seconds := duration / time.Second

	if days > 0 {
		return fmt.Sprintf("%dd%dh", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
