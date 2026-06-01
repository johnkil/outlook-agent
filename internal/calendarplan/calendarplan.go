package calendarplan

import (
	"sort"
	"strings"
	"time"
)

const (
	TentativeBusy   = "busy"
	TentativeFree   = "free"
	DefaultDuration = 30 * time.Minute
)

type Interval struct {
	Start  time.Time
	End    time.Time
	Status string
}

type Suggestion struct {
	Start time.Time
	End   time.Time
}

type Options struct {
	Duration        time.Duration
	Step            time.Duration
	MaxSuggestions  int
	TentativePolicy string
}

func FindSuggestions(start time.Time, end time.Time, busy []Interval, options Options) []Suggestion {
	duration := options.Duration
	if duration <= 0 {
		duration = DefaultDuration
	}
	step := options.Step
	if step <= 0 {
		step = 30 * time.Minute
	}
	maxSuggestions := options.MaxSuggestions
	if maxSuggestions <= 0 {
		maxSuggestions = 5
	}
	blocking := mergeIntervals(blockingIntervals(busy, options.TentativePolicy))
	suggestions := make([]Suggestion, 0, maxSuggestions)
	for slotStart := start; !slotStart.Add(duration).After(end); slotStart = slotStart.Add(step) {
		slotEnd := slotStart.Add(duration)
		if overlapsAny(slotStart, slotEnd, blocking) {
			continue
		}
		suggestions = append(suggestions, Suggestion{Start: slotStart, End: slotEnd})
		if len(suggestions) == maxSuggestions {
			break
		}
	}
	return suggestions
}

func DurationFromMinutes(minutes float64) time.Duration {
	if minutes <= 0 {
		return DefaultDuration
	}
	duration := time.Duration(minutes * float64(time.Minute))
	if duration <= 0 {
		return DefaultDuration
	}
	return duration
}

func blockingIntervals(intervals []Interval, tentativePolicy string) []Interval {
	blocking := make([]Interval, 0, len(intervals))
	tentativeIsFree := strings.EqualFold(strings.TrimSpace(tentativePolicy), TentativeFree)
	for _, interval := range intervals {
		if !interval.End.After(interval.Start) {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(interval.Status))
		switch status {
		case "busy", "oof", "outofoffice":
			blocking = append(blocking, interval)
		case "tentative":
			if !tentativeIsFree {
				blocking = append(blocking, interval)
			}
		}
	}
	return blocking
}

func mergeIntervals(intervals []Interval) []Interval {
	if len(intervals) <= 1 {
		return intervals
	}
	sort.Slice(intervals, func(left int, right int) bool {
		return intervals[left].Start.Before(intervals[right].Start)
	})
	merged := []Interval{intervals[0]}
	for _, interval := range intervals[1:] {
		last := &merged[len(merged)-1]
		if interval.Start.After(last.End) {
			merged = append(merged, interval)
			continue
		}
		if interval.End.After(last.End) {
			last.End = interval.End
		}
	}
	return merged
}

func overlapsAny(start time.Time, end time.Time, intervals []Interval) bool {
	for _, interval := range intervals {
		if start.Before(interval.End) && end.After(interval.Start) {
			return true
		}
	}
	return false
}
