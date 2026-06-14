package notification

import (
	"fmt"
	"strings"
	"time"
)

// resolveTemplate stringifies and replaces placeholders like {{payload.key}} or nested {{payload.order.id}}
func resolveTemplate(tpl string, payload map[string]any) string {
	flat := make(map[string]string)
	flattenMap(payload, "payload", flat)

	for k, v := range flat {
		placeholder := fmt.Sprintf("{{%s}}", k)
		tpl = strings.ReplaceAll(tpl, placeholder, v)
	}
	return tpl
}

func flattenMap(m map[string]any, prefix string, result map[string]string) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case map[string]any:
			flattenMap(val, key, result)
		case map[string]string:
			temp := make(map[string]any)
			for k2, v2 := range val {
				temp[k2] = v2
			}
			flattenMap(temp, key, result)
		default:
			result[key] = fmt.Sprintf("%v", v)
		}
	}
}

// parseCronAndGetNext calculates the next time matching the 5-field cron expression after `from`
func parseCronAndGetNext(expr string, from time.Time) (time.Time, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return time.Time{}, fmt.Errorf("cron expression must contain exactly 5 fields")
	}

	// Round to nearest minute and step forward by 1 minute
	current := from.Truncate(time.Minute).Add(time.Minute)
	limit := from.AddDate(1, 0, 0) // Max 1 year search limit

	for current.Before(limit) {
		match, err := matchCronFields(fields, current)
		if err != nil {
			return time.Time{}, err
		}
		if match {
			return current, nil
		}
		current = current.Add(time.Minute)
	}

	return time.Time{}, fmt.Errorf("no matching execution time found within 1 year")
}

func matchCronFields(fields []string, t time.Time) (bool, error) {
	minMatch, err := matchField(fields[0], t.Minute(), 0, 59)
	if err != nil || !minMatch {
		return false, err
	}
	hourMatch, err := matchField(fields[1], t.Hour(), 0, 23)
	if err != nil || !hourMatch {
		return false, err
	}
	domMatch, err := matchField(fields[2], t.Day(), 1, 31)
	if err != nil || !domMatch {
		return false, err
	}
	monthMatch, err := matchField(fields[3], int(t.Month()), 1, 12)
	if err != nil || !monthMatch {
		return false, err
	}
	// Day of week (0-6, Sunday = 0)
	dowMatch, err := matchField(fields[4], int(t.Weekday()), 0, 6)
	if err != nil || !dowMatch {
		return false, err
	}

	return true, nil
}

func matchField(rule string, val int, minVal int, maxVal int) (bool, error) {
	if rule == "*" {
		return true, nil
	}

	// Handle steps: */5
	if strings.HasPrefix(rule, "*/") {
		var step int
		_, err := fmt.Sscanf(rule, "*/%d", &step)
		if err != nil {
			return false, fmt.Errorf("invalid step pattern: %w", err)
		}
		if step <= 0 {
			return false, fmt.Errorf("step must be positive")
		}
		return val % step == 0, nil
	}

	// Handle comma-separated list: 1,5,10-15
	parts := strings.Split(rule, ",")
	for _, part := range parts {
		if strings.Contains(part, "-") {
			var rMin, rMax int
			_, err := fmt.Sscanf(part, "%d-%d", &rMin, &rMax)
			if err != nil {
				return false, fmt.Errorf("invalid range pattern: %w", err)
			}
			if val >= rMin && val <= rMax {
				return true, nil
			}
		} else {
			var exact int
			_, err := fmt.Sscanf(part, "%d", &exact)
			if err == nil && exact == val {
				return true, nil
			}
		}
	}

	return false, nil
}
