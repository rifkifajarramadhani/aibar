package claude

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/overhaul/aibar/internal/usage"
)

const (
	fiveHourMinutes = 300
	weeklyMinutes   = 10080
	maxUsageBody    = 1 << 20
)

var ErrNoWindows = errors.New("claude usage response contains no supported windows")

type usageWindow struct {
	Utilization    *float64        `json:"utilization"`
	UsedPercentage *float64        `json:"used_percentage"`
	ResetsAt       json.RawMessage `json:"resets_at"`
}

func ParseUsage(reader io.Reader, now time.Time) (usage.Snapshot, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxUsageBody+1))
	if err != nil {
		return usage.Snapshot{}, err
	}

	if len(data) > maxUsageBody {
		return usage.Snapshot{}, errors.New("claude usage response is too large")
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return usage.Snapshot{}, fmt.Errorf("decode claude usage response: %w", err)
	}

	if nested, ok := root["usage"]; ok {
		var usage map[string]json.RawMessage
		if err := json.Unmarshal(nested, &usage); err == nil {
			for key, value := range usage {
				root[key] = value
			}
		}
	}

	windows := make([]usage.Window, 0, 2)
	for _, candidate := range []struct {
		keys          []string
		label         string
		windowMinutes int
	}{
		{keys: []string{"five_hour"}, label: "5h", windowMinutes: fiveHourMinutes},
		{keys: []string{"seven_day", "weekly"}, label: "weekly", windowMinutes: weeklyMinutes},
	} {
		var raw json.RawMessage
		for _, key := range candidate.keys {
			if value, ok := root[key]; ok {
				raw = value
				break
			}
		}

		if len(raw) == 0 {
			continue
		}

		var parsed usageWindow
		if err := json.Unmarshal(raw, &parsed); err != nil {
			continue
		}

		used, ok := parsed.percentage()
		if !ok {
			continue
		}

		resetsAt := parseResetTime(parsed.ResetsAt)
		windows = append(windows, usage.Window{Label: candidate.label, UsedPct: clampPercentage(used), ResetsAt: resetsAt, WindowMinutes: candidate.windowMinutes})
	}

	if len(windows) == 0 {
		return usage.Snapshot{}, ErrNoWindows
	}

	sort.Slice(windows, func(i, j int) bool { return windows[i].Label < windows[j].Label })

	return usage.Snapshot{Windows: windows, FetchedAt: now}, nil
}

func (w usageWindow) percentage() (float64, bool) {
	if w.Utilization != nil && !math.IsNaN(*w.Utilization) && !math.IsInf(*w.Utilization, 0) {
		return *w.Utilization, true
	}

	if w.UsedPercentage != nil && !math.IsNaN(*w.UsedPercentage) && !math.IsInf(*w.UsedPercentage, 0) {
		return *w.UsedPercentage, true
	}

	return 0, false
}

func parseResetTime(raw json.RawMessage) time.Time {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return time.Time{}
	}

	var text string
	if json.Unmarshal(raw, &text) == nil {
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
			if parsed, err := time.Parse(layout, text); err == nil {
				return parsed
			}
		}
	}

	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&number) == nil {
		value, err := strconv.ParseInt(number.String(), 10, 64)
		if err == nil {
			if value > 1_000_000_000_000 {
				return time.UnixMilli(value)
			}

			return time.Unix(value, 0)
		}
	}

	return time.Time{}
}

func clampPercentage(value float64) float64 {
	if value < 0 {
		return 0
	}

	if value > 100 {
		return 100
	}

	return value
}

type UsageTotals struct {
	InputTokens              int64
	OutputTokens             int64
	CacheCreationInputTokens int64
	CacheReadInputTokens     int64
}

func (u UsageTotals) HasUsage() bool {
	return u.InputTokens != 0 || u.OutputTokens != 0 || u.CacheCreationInputTokens != 0 || u.CacheReadInputTokens != 0
}

func ParseProjectUsage(reader io.Reader) (UsageTotals, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	var totals UsageTotals

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var record projectRecord
		if err := json.Unmarshal(line, &record); err != nil {
			continue
		}

		if record.Type != "assistant" || record.Message == nil || record.Message.Role != "assistant" || record.Message.Usage == nil {
			continue
		}

		totals.InputTokens += record.Message.Usage.InputTokens
		totals.OutputTokens += record.Message.Usage.OutputTokens
		totals.CacheCreationInputTokens += record.Message.Usage.CacheCreationInputTokens
		totals.CacheReadInputTokens += record.Message.Usage.CacheReadInputTokens
	}

	if err := scanner.Err(); err != nil {
		return UsageTotals{}, err
	}

	return totals, nil
}

type projectRecord struct {
	Type    string          `json:"type"`
	Message *projectMessage `json:"message"`
}

type projectMessage struct {
	Role  string      `json:"role"`
	Usage *tokenUsage `json:"usage"`
}

type tokenUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
}
