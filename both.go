package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"google.golang.org/api/calendar/v3"
)

type merge struct {
	tw  *taskWarrior
	cal *calendarClient
	ctx *cliContext

	unsyncedEvents    []*calendar.Event
	eventsIDMap       map[string]*calendar.Event
	unsyncedTasks     taskWarriorItems
	checkTasks        taskWarriorItems
	deletedTasks      taskWarriorItems
	syncedTasks       taskWarriorItems
	deletedTaskCalMap map[string]struct{}
	taskIDMap         map[string]*taskWarriorItem
}

func newMerge(ctx *cliContext, tw *taskWarrior, cal *calendarClient) *merge {
	return &merge{
		tw:  tw,
		cal: cal,
		ctx: ctx,

		unsyncedEvents:    []*calendar.Event{},
		eventsIDMap:       map[string]*calendar.Event{},
		unsyncedTasks:     taskWarriorItems{},
		checkTasks:        taskWarriorItems{},
		deletedTasks:      taskWarriorItems{},
		syncedTasks:       taskWarriorItems{},
		deletedTaskCalMap: map[string]struct{}{},
		taskIDMap:         map[string]*taskWarriorItem{},
	}
}

func (m *merge) run() error {
	tags, err := m.ctx.makeTags()
	if err != nil {
		return err
	}

	tasks, err := m.tw.exportTasksByCommand(append([]string{"export"}, tags.decorate()...)...)
	if err != nil {
		return err
	}

	t1, t2 := m.ctx.getTimeWindow()

	events, err := m.cal.gatherEvents(t1, t2)
	if err != nil {
		return fmt.Errorf("Trouble gathering events: %w", err)
	}

	for _, event := range events.Items {
		m.eventsIDMap[event.Id] = event
	}

	for _, task := range tasks {
		if task.Status == "deleted" || task.Status == "completed" {
			if _, ok := m.eventsIDMap[task.CalendarID]; ok && task.CalendarID != "" {
				m.deletedTasks = append(m.deletedTasks, task)
				m.deletedTaskCalMap[task.CalendarID] = struct{}{}
			}
		}

		if task.CalendarID == "" {
			m.unsyncedTasks = append(m.unsyncedTasks, task)
		} else {
			m.checkTasks = append(m.checkTasks, task)
		}
	}

	for _, task := range m.checkTasks {
		m.taskIDMap[task.CalendarID] = task
	}

	for _, event := range events.Items {
		id := event.Id
		task, ok := m.taskIDMap[id]
		if !ok {
			m.unsyncedEvents = append(m.unsyncedEvents, event)
		} else {
			modified, err := unify(task, event)
			if err != nil {
				return err
			}

			if modified {
				m.checkTasks = append(m.checkTasks, task)
			}
		}
	}

	for _, event := range m.unsyncedEvents {
		id := event.Id
		if _, ok := m.deletedTaskCalMap[id]; ok {
			continue
		}

		fmt.Printf("Syncing %q (%.20q) to taskwarrior\n", event.Id, event.Summary)
		due, err := eventDue(event)
		if err != nil {
			return err
		}

		m.syncedTasks = append(m.syncedTasks, &taskWarriorItem{
			Description: strings.TrimSpace(event.Summary),
			Status:      "pending",
			Entry:       toTaskWarriorTime(time.Now()),
			Due:         due,
			Tags:        tags,
			CalendarID:  event.Id,
		})
	}

	for _, task := range m.checkTasks {
		due, err := taskTimeToGCal(task.Due)
		if err != nil {
			return fmt.Errorf("Task %q (%.20q) Could not convert due time to gcal event time: %w", task.UUID, task.Description, err)
		}

		var action bool

		if task.CalendarID != "" {
			event, err := m.cal.getEvent(task.CalendarID)
			if err != nil {
				fmt.Printf("Could not retrieve event for calendar ID %q: %v\n", task.CalendarID, err)
				continue
			}

			modified, err := unify(task, event)
			if err != nil {
				return fmt.Errorf("Error reconciling task and event: %w", err)
			}

			if modified {
				event, err = m.cal.modifyEvent(event)
				if err != nil {
					return fmt.Errorf("Error modifying calendar event: %w", err)
				}
			}

			action = modified
		} else {
			var err error

			event, err := m.cal.insertEvent(&calendar.Event{
				Summary: task.Description,
				Start:   due,
				End:     due,
			})
			if err != nil {
				return fmt.Errorf("Error inserting calendar event: %w", err)
			}

			modified, err := unify(task, event)
			if err != nil {
				return fmt.Errorf("Error reconciling task and event: %w", err)
			}

			action = modified
		}

		if action {
			fmt.Printf("Pushing %q (%.20q) to gcal\n", task.UUID, task.Description)
			m.checkTasks = append(m.checkTasks, task)
		}
	}

	for _, task := range m.unsyncedTasks {
		due, err := taskTimeToGCal(task.Due)
		if err != nil {
			return fmt.Errorf("Task %q (%.20q) Could not convert due time to gcal event time: %w", task.UUID, task.Description, err)
		}

		event, err := m.cal.insertEvent(&calendar.Event{
			Summary: task.Description,
			Start:   due,
			End:     due,
		})
		if err != nil {
			return fmt.Errorf("Error inserting calendar event: %w", err)
		}

		modified, err := unify(task, event)
		if err != nil {
			return fmt.Errorf("Error reconciling task and event: %w", err)
		}

		if modified {
			m.checkTasks = append(m.checkTasks, task)
		}
	}

	for _, task := range m.deletedTasks {
		if err := m.cal.deleteEvent(task.CalendarID); err == nil {
			fmt.Printf("Deleting task %q (%.20q)\n", task.UUID, task.Description)
		}
	}

	if err := m.tw.importTasks(m.checkTasks); err != nil {
		return fmt.Errorf("Could not import tasks: %w", err)
	}

	return nil
}

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

func unify(task *taskWarriorItem, event *calendar.Event) (bool, error) {
	if task == nil || event == nil {
		return false, errors.New("nil event or task passed")
	}

	modified := false
	taskNewer := true

	taskModified, err := task.Modified.ToTime()
	if err != nil {
		taskModified = time.Time{}
	}

	var calModified time.Time

	ct, err := gcalTimeToTaskWarrior(event.Updated)
	if err != nil {
		calModified = time.Time{}
	} else {
		calModified, err = ct.ToTime()
		if err != nil {
			calModified = time.Time{}
		}
	}

	if taskModified.Before(calModified) {
		taskNewer = false
	}

	if task.CalendarID != event.Id {
		task.CalendarID = event.Id
		modified = true
	}

	if strings.TrimSpace(task.Description) != strings.TrimSpace(event.Summary) {
		if taskNewer {
			event.Summary = strings.TrimSpace(task.Description)
			task.Description = event.Summary
		} else {
			task.Description = strings.TrimSpace(event.Summary)
			event.Summary = task.Description
		}
		modified = true
	}

	due, err := eventDue(event)
	if err != nil {
		return false, err
	}

	if due != task.Due {
		if taskNewer {
			var err error
			event.Start, err = taskTimeToGCal(task.Due)
			event.End = event.Start
			if err != nil {
				return false, fmt.Errorf("Task %q could not assign start time to calendar id %q: %w", task.UUID, event.Id, err)
			}
		} else {
			task.Due = due
		}

		modified = true
	}

	return modified, nil
}
