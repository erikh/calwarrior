package main

import (
	"fmt"
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
)

type calendarClient struct {
	*calendar.Service
}

func getCalendarClient() (*calendarClient, error) {
	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(CredentialJSON, calendar.CalendarScope)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse client secret file to config: %w", err)
	}

	client, err := getClient(config)
	if err != nil {
		return nil, fmt.Errorf("Trouble while gathering client information: %w", err)
	}

	srv, err := calendar.New(client)
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve Calendar client: %w", err)
	}

	return &calendarClient{srv}, nil
}

func (cal *calendarClient) gatherEvents(t1, t2 time.Time) (*calendar.Events, error) {
	return cal.Events.List("primary").ShowDeleted(false).
		SingleEvents(true).TimeMin(toCalendarTime(t1)).TimeMax(toCalendarTime(t2)).OrderBy("startTime").Do()
}

func (cal *calendarClient) insertEvent(event *calendar.Event) (*calendar.Event, error) {
	return cal.Events.Insert("primary", event).Do()
}

func (cal *calendarClient) modifyEvent(event *calendar.Event) (*calendar.Event, error) {
	return cal.Events.Patch("primary", event.Id, event).Do()
}

func (cal *calendarClient) getEvent(calID string) (*calendar.Event, error) {
	return cal.Events.Get("primary", calID).Do()
}

func (cal *calendarClient) deleteEvent(calID string) error {
	return cal.Events.Delete("primary", calID).Do()
}
