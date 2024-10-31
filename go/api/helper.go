package api

import (
	"fmt"
	"time"
)

const (
	DateFormat = "2006-01-02"
	TimeFormat = "15:04:05"
)

var timezoneLocation *time.Location

func init() {
	timezoneLocation = TimezoneLocation
}

func SetTimezoneLocation(regionName string) (err error) {
	timezoneLocation, err = time.LoadLocation(regionName)
	return
}

func GetTimeNowWithTimezone() time.Time {
	return time.Now().In(timezoneLocation)
}

func GetDateTimeNowStringWithFormat(layout string) string {
	return GetTimeNowWithTimezone().Format(layout)
}

func GetDateTimeNowStringWithTimezone() string {
	return GetTimeNowWithTimezone().Format(fmt.Sprintf("%s %s", DateFormat, TimeFormat))
}

func GetTimeNowStringWithTimezone() string {
	return GetTimeNowWithTimezone().Format(TimeFormat)
}

func GetDateNowStringWithTimezone() string {
	return GetTimeNowWithTimezone().Format(DateFormat)
}

func ParseToDateOnly(date string) string {
	dateParse, _ := time.Parse(time.RFC3339, date)

	return dateParse.Format("2006-01-02")
}

func ParseToTimeOnly(t string) string {
	dateParse, _ := time.Parse(time.RFC3339, t)
	return dateParse.Format("15:04:05")
}

// ParseDate takes a date string and tries to parse it into the standard format "YYYY-MM-DD".
func ParseDate(dateStrInput string) (string, error) {
	// List of date formats to try
	dateFormats := []string{
		"2006-01-02",      // YYYY-MM-DD
		"02-01-2006",      // DD-MM-YYYY
		"02/01/2006",      // DD/MM/YYYY
		"01/02/2006",      // MM/DD/YYYY
		"January 2, 2006", // Month DD, YYYY
		"2 Jan 2006",      // DD Mon YYYY
		"2006.01.02",      // YYYY.MM.DD
		// Add more formats if needed
	}

	for _, format := range dateFormats {
		t, err := time.Parse(format, dateStrInput)
		if err == nil {
			return t.Format(DateFormat), nil
		}
	}

	// Return an error if no formats matched
	return "", fmt.Errorf("unable to parse date: %s", dateStrInput)
}
