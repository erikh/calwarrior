package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/urfave/cli/v2"
	"google.golang.org/api/calendar/v3"
)

type cliContext struct {
	*cli.Context
}

func (ctx *cliContext) makeTags() (taskWarriorTags, error) {
	slice := ctx.StringSlice("tag")
	if len(slice) == 0 {
		return nil, errors.New("at least one tag is required")
	}

	return taskWarriorTags(slice), nil
}

func (ctx *cliContext) getTimeWindow() (time.Time, time.Time) {
	t1 := time.Now().Add(-time.Hour * 24)
	t2 := t1.Add(ctx.Duration("duration"))

	return t1, t2
}

func (ctx *cliContext) run() error {
	tags, err := ctx.makeTags()
	if err != nil {
		return err
	}

	tw, err := findTaskwarrior()
	if err != nil {
		return err
	}

	tasks, err := tw.exportTasksByCommand(append([]string{"export"}, tags.decorate()...)...)
	if err != nil {
		return err
	}

	cal, err := getCalendarClient()
	if err != nil {
		return fmt.Errorf("Trouble contacting google calendar: %w", err)
	}

	eventsIDMap := map[string]*calendar.Event{}

	t1, t2 := ctx.getTimeWindow()

	events, err := cal.gatherEvents(t1, t2)
	if err != nil {
		return fmt.Errorf("Trouble gathering events: %w", err)
	}

	for _, event := range events.Items {
		eventsIDMap[event.Id] = event
	}

	unsyncedTasks := taskWarriorItems{}
	checkTasks := taskWarriorItems{}
	deletedTasks := taskWarriorItems{}
	deletedTaskCalMap := map[string]struct{}{}

	for _, task := range tasks {
		if task.Status == "deleted" || task.Status == "completed" {
			if _, ok := eventsIDMap[task.CalendarID]; ok && task.CalendarID != "" {
				deletedTasks = append(deletedTasks, task)
				deletedTaskCalMap[task.CalendarID] = struct{}{}
			}
		}

		if task.CalendarID == "" {
			unsyncedTasks = append(unsyncedTasks, task)
		} else {
			checkTasks = append(checkTasks, task)
		}
	}

	taskIDMap := map[string]*taskWarriorItem{}

	for _, task := range checkTasks {
		taskIDMap[task.CalendarID] = task
	}

	unsyncedEvents := []*calendar.Event{}

	for _, event := range events.Items {
		id := event.Id
		task, ok := taskIDMap[id]
		if !ok {
			unsyncedEvents = append(unsyncedEvents, event)
		} else {
			modified, err := unify(task, event)
			if err != nil {
				return err
			}

			if modified {
				checkTasks = append(checkTasks, task)
			}
		}
	}

	syncedTasks := taskWarriorItems{}

	for _, event := range unsyncedEvents {
		id := event.Id
		if _, ok := deletedTaskCalMap[id]; ok {
			continue
		}

		fmt.Printf("Syncing %q (%.20q) to taskwarrior\n", event.Id, event.Summary)
		due, err := eventDue(event)
		if err != nil {
			return err
		}

		syncedTasks = append(syncedTasks, &taskWarriorItem{
			Description: event.Summary,
			Status:      "pending",
			Entry:       toTaskWarriorTime(time.Now()),
			Due:         due,
			Tags:        tags,
			CalendarID:  event.Id,
		})
	}

	for _, task := range checkTasks {
		due, err := taskTimeToGCal(task.Due)
		if err != nil {
			return fmt.Errorf("Task %q (%.20q) Could not convert due time to gcal event time: %w", task.UUID, task.Description, err)
		}

		var action bool

		if task.CalendarID != "" {
			event, err := cal.getEvent(task.CalendarID)
			if err != nil {
				fmt.Printf("Could not retrieve event for calendar ID %q: %v\n", task.CalendarID, err)
				continue
			}

			m, err := unify(task, event)
			if err != nil {
				return fmt.Errorf("Error reconciling task and event: %w", err)
			}

			if m {
				event, err = cal.modifyEvent(event)
				if err != nil {
					return fmt.Errorf("Error modifying calendar event: %w", err)
				}
			}

			action = m
		} else {
			var err error

			event, err := cal.insertEvent(&calendar.Event{
				Summary: task.Description,
				Start:   due,
				End:     due,
			})
			if err != nil {
				return fmt.Errorf("Error inserting calendar event: %w", err)
			}

			m, err := unify(task, event)
			if err != nil {
				return fmt.Errorf("Error reconciling task and event: %w", err)
			}

			action = m
		}

		if action {
			fmt.Printf("Pushing %q (%.20q) to gcal\n", task.UUID, task.Description)
			checkTasks = append(checkTasks, task)
		}
	}

	for _, task := range unsyncedTasks {
		due, err := taskTimeToGCal(task.Due)
		if err != nil {
			return fmt.Errorf("Task %q (%.20q) Could not convert due time to gcal event time: %w", task.UUID, task.Description, err)
		}

		event, err := cal.insertEvent(&calendar.Event{
			Summary: task.Description,
			Start:   due,
			End:     due,
		})
		if err != nil {
			return fmt.Errorf("Error inserting calendar event: %w", err)
		}

		m, err := unify(task, event)
		if err != nil {
			return fmt.Errorf("Error reconciling task and event: %w", err)
		}

		if m {
			checkTasks = append(checkTasks, task)
		}
	}

	for _, task := range deletedTasks {
		if err := cal.deleteEvent(task.CalendarID); err == nil {
			fmt.Printf("Deleting task %q (%.20q)\n", task.UUID, task.Description)
		}
	}

	if err := tw.importTasks(checkTasks); err != nil {
		return fmt.Errorf("Could not import tasks: %w", err)
	}

	return nil
}
