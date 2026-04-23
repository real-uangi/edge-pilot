package application

import (
	"testing"
	"time"
)

func TestNextCronTimeUTC_EveryFiveMinutes(t *testing.T) {
	from := time.Date(2026, 4, 23, 8, 7, 33, 0, time.UTC)
	next, err := nextCronTimeUTC("*/5 * * * *", from)
	if err != nil {
		t.Fatalf("nextCronTimeUTC() error = %v", err)
	}
	expected := time.Date(2026, 4, 23, 8, 10, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Fatalf("nextCronTimeUTC() = %v, want %v", next, expected)
	}
}

func TestNextCronTimeUTC_WithDowNames(t *testing.T) {
	from := time.Date(2026, 4, 23, 8, 0, 0, 0, time.UTC) // Thu
	next, err := nextCronTimeUTC("0 9 * * FRI", from)
	if err != nil {
		t.Fatalf("nextCronTimeUTC() error = %v", err)
	}
	expected := time.Date(2026, 4, 24, 9, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Fatalf("nextCronTimeUTC() = %v, want %v", next, expected)
	}
}

func TestParseCronSchedule_InvalidFieldCount(t *testing.T) {
	_, err := parseCronSchedule("*/5 * * *")
	if err == nil {
		t.Fatalf("expected error")
	}
}
