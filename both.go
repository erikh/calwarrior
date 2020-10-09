package main

import (
	"errors"
	"time"

	"google.golang.org/api/calendar/v3"
)

func eventDue(event *calendar.Event) (taskWarriorTime, error) {
	if event == nil {
		return "", errors.New("event is nil")
	}

	var (
		due taskWarriorTime
		err error
	)

	if event.Start.DateTime != "" {
		due, err = gcalTimeToTaskWarrior(event.Start.DateTime)
	} else if event.Start.Date != "" {
		due, err = gcalDateToTaskWarrior(event.Start.Date)
	} else {
		err = errors.New("no date to convert")
	}
	if err != nil {
		return "", err
	}

	return due, nil
}

func gcalTimeToTaskWarrior(gcal string) (taskWarriorTime, error) {
	parsed, err := time.ParseInLocation(time.RFC3339, gcal, time.Local)
	if err != nil {
		return "", err
	}

	return taskWarriorTime(toTaskWarriorTime(parsed.In(time.UTC))), nil
}

func gcalDateToTaskWarrior(gcal string) (taskWarriorTime, error) {
	parsed, err := time.ParseInLocation("2006-01-02", gcal, time.Local)
	if err != nil {
		return "", err
	}

	return taskWarriorTime(toTaskWarriorTime(parsed.In(time.UTC))), nil
}

func taskTimeToGCal(tw taskWarriorTime) (*calendar.EventDateTime, error) {
	t, err := tw.ToTime()
	if err != nil {
		return nil, err
	}

	return &calendar.EventDateTime{DateTime: toCalendarTime(t), TimeZone: time.Local.String()}, nil
}
