package application

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type cronSchedule struct {
	minutes map[int]struct{}
	hours   map[int]struct{}
	doms    map[int]struct{}
	months  map[int]struct{}
	dows    map[int]struct{}
}

func parseCronSchedule(expr string) (*cronSchedule, error) {
	parts := strings.Fields(strings.TrimSpace(expr))
	if len(parts) != 5 {
		return nil, fmt.Errorf("cron expr must have 5 fields")
	}
	minutes, err := parseCronField(parts[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("invalid minute field: %w", err)
	}
	hours, err := parseCronField(parts[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("invalid hour field: %w", err)
	}
	doms, err := parseCronField(parts[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("invalid day-of-month field: %w", err)
	}
	months, err := parseCronField(parts[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("invalid month field: %w", err)
	}
	dows, err := parseCronField(parts[4], 0, 6)
	if err != nil {
		if normalized, nErr := normalizeDow(parts[4]); nErr == nil {
			dows, err = parseCronField(normalized, 0, 6)
		}
		if err != nil {
			return nil, fmt.Errorf("invalid day-of-week field: %w", err)
		}
	}
	return &cronSchedule{minutes: minutes, hours: hours, doms: doms, months: months, dows: dows}, nil
}

func normalizeDow(raw string) (string, error) {
	mapping := map[string]string{
		"SUN": "0",
		"MON": "1",
		"TUE": "2",
		"WED": "3",
		"THU": "4",
		"FRI": "5",
		"SAT": "6",
	}
	parts := strings.Split(strings.ToUpper(strings.TrimSpace(raw)), ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		v := strings.TrimSpace(part)
		if v == "" {
			continue
		}
		if mapped, ok := mapping[v]; ok {
			out = append(out, mapped)
			continue
		}
		if strings.Contains(v, "-") {
			rangeParts := strings.Split(v, "-")
			if len(rangeParts) != 2 {
				return "", fmt.Errorf("invalid day-of-week token: %s", v)
			}
			left, leftOk := mapping[rangeParts[0]]
			right, rightOk := mapping[rangeParts[1]]
			if !leftOk || !rightOk {
				return "", fmt.Errorf("invalid day-of-week token: %s", v)
			}
			out = append(out, left+"-"+right)
			continue
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return "", fmt.Errorf("invalid day-of-week")
	}
	return strings.Join(out, ","), nil
}

func parseCronField(expr string, min int, max int) (map[int]struct{}, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, fmt.Errorf("empty field")
	}
	result := make(map[int]struct{})
	segments := strings.Split(expr, ",")
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		if err := addCronSegment(result, seg, min, max); err != nil {
			return nil, err
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no valid values")
	}
	return result, nil
}

func addCronSegment(out map[int]struct{}, seg string, min int, max int) error {
	step := 1
	base := seg
	if strings.Contains(seg, "/") {
		parts := strings.Split(seg, "/")
		if len(parts) != 2 {
			return fmt.Errorf("invalid step syntax")
		}
		base = parts[0]
		v, err := strconv.Atoi(parts[1])
		if err != nil || v <= 0 {
			return fmt.Errorf("invalid step value")
		}
		step = v
	}
	rangeStart := min
	rangeEnd := max
	switch {
	case base == "*" || base == "":
		// keep defaults
	case strings.Contains(base, "-"):
		parts := strings.Split(base, "-")
		if len(parts) != 2 {
			return fmt.Errorf("invalid range syntax")
		}
		a, err := strconv.Atoi(parts[0])
		if err != nil {
			return fmt.Errorf("invalid range start")
		}
		b, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("invalid range end")
		}
		rangeStart = a
		rangeEnd = b
	default:
		v, err := strconv.Atoi(base)
		if err != nil {
			return fmt.Errorf("invalid value")
		}
		rangeStart = v
		rangeEnd = v
	}
	if rangeStart < min || rangeEnd > max || rangeStart > rangeEnd {
		return fmt.Errorf("out of range")
	}
	for i := rangeStart; i <= rangeEnd; i += step {
		out[i] = struct{}{}
	}
	return nil
}

func nextCronTimeUTC(expr string, from time.Time) (time.Time, error) {
	schedule, err := parseCronSchedule(expr)
	if err != nil {
		return time.Time{}, err
	}
	current := from.UTC().Truncate(time.Minute).Add(time.Minute)
	deadline := current.Add(366 * 24 * time.Hour)
	for !current.After(deadline) {
		if _, ok := schedule.minutes[current.Minute()]; !ok {
			current = current.Add(time.Minute)
			continue
		}
		if _, ok := schedule.hours[current.Hour()]; !ok {
			current = current.Add(time.Minute)
			continue
		}
		if _, ok := schedule.months[int(current.Month())]; !ok {
			current = current.Add(time.Minute)
			continue
		}
		if _, ok := schedule.doms[current.Day()]; !ok {
			current = current.Add(time.Minute)
			continue
		}
		if _, ok := schedule.dows[int(current.Weekday())]; !ok {
			current = current.Add(time.Minute)
			continue
		}
		return current, nil
	}
	return time.Time{}, fmt.Errorf("failed to find next cron time within 366 days")
}
