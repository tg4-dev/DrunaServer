package handler

import (
	"fmt"
	"time"
)

func parseCalendarDate(dateStr, timezone string) (time.Time, error) {
	if timezone == "" {
		timezone = "UTC"
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timezone")
	}
	parsed, err := time.ParseInLocation("2006-01-02", dateStr, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("date must be in YYYY-MM-DD format")
	}
	return parsed, nil
}
