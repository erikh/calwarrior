package main

import (
	"errors"
	"time"

	"google.golang.org/api/calendar/v3"
)

type (
	calendarDate string
	calendarTime string
)

type calendarPeriod interface {
	ToTaskWarriorTime() (taskWarriorTime, error)
}

func toCalendarTime(t time.Time) calendarTime {
	return calendarTime(t.Format(time.RFC3339))
}

func toCalendarDate(t time.Time) calendarDate {
	return calendarDate(t.Format(time.RFC3339))
}

func eventToCalendarPeriod(event *calendar.Event) (calendarPeriod, error) {
	var (
		period calendarPeriod
		err    error
	)

	if event.Start.DateTime != "" {
		period = calendarTime(event.Start.DateTime)
	} else if event.Start.Date != "" {
		period = calendarDate(event.Start.Date)
	} else {
		err = errors.New("no date to convert")
	}
	if err != nil {
		return period, err
	}

	return period, nil
}

func eventDue(event *calendar.Event) (taskWarriorTime, error) {
	if event == nil {
		return "", errors.New("event is nil")
	}

	period, err := eventToCalendarPeriod(event)
	if err != nil {
		return "", err
	}

	return period.ToTaskWarriorTime()
}

func (c calendarTime) ToTaskWarriorTime() (taskWarriorTime, error) {
	parsed, err := time.ParseInLocation(time.RFC3339, string(c), time.Local)
	if err != nil {
		return "", err
	}

	return toTaskWarriorTime(parsed.In(time.UTC)), nil
}

func (c calendarDate) ToTaskWarriorTime() (taskWarriorTime, error) {
	parsed, err := time.ParseInLocation("2006-01-02", string(c), time.Local)
	if err != nil {
		return "", err
	}

	return toTaskWarriorTime(parsed.In(time.UTC)), nil
}
