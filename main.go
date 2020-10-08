package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/urfave/cli/v2"
	"google.golang.org/api/calendar/v3"
)

func main() {
	app := cli.NewApp()
	app.Authors = []*cli.Author{{Name: "Erik Hollensbe", Email: "erik+github@hollensbe.org"}}
	app.Usage = "Synchronize Google Calendar and Taskwarrior"
	app.UsageText = filepath.Base(os.Args[0]) + " [--flags or help]"

	app.Flags = []cli.Flag{
		&cli.DurationFlag{
			Name:  "duration, d",
			Usage: "Upcoming items to monitor in google calendar. Keeping this small and polling frequently is better",
			Value: 7 * 24 * time.Hour,
		},
		&cli.StringSliceFlag{
			Name:  "tag, t",
			Usage: "Tag new items coming from google calendar with the specified tag(s).",
			Value: cli.NewStringSlice("calendar"),
		},
	}

	app.Action = run

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func makeTags(ctx *cli.Context) ([]string, []string, error) {
	slice := ctx.StringSlice("tag")
	if len(slice) == 0 {
		return nil, nil, errors.New("at least one tag is required")
	}

	tags := []string{}

	for i := range slice {
		tags = append(tags, "+"+slice[i])
	}

	return slice, tags, nil
}

func run(ctx *cli.Context) error {
	tw, err := findTaskwarrior()
	if err != nil {
		return err
	}

	slice, tags, err := makeTags(ctx)
	if err != nil {
		return err
	}

	t1 := time.Now().Add(-time.Hour * 24)
	t2 := t1.Add(ctx.Duration("duration"))

	tasks, err := tw.exportTasksByCommand(append([]string{"export"}, tags...)...)
	if err != nil {
		return err
	}

	srv, err := getCalendarClient()
	if err != nil {
		return fmt.Errorf("Trouble contacting google calendar: %w", err)
	}

	eventsIDMap := map[string]*calendar.Event{}

	events, err := gatherEvents(srv, t1, t2)
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
			Tags:        slice,
			CalendarID:  event.Id,
		})
	}

	if err := tw.importTasks(syncedTasks); err != nil {
		return fmt.Errorf("Could not import tasks: %w", err)
	}

	resyncTasks := taskWarriorItems{}

	for _, task := range checkTasks {
		due, err := taskTimeToGCal(task.Due)
		if err != nil {
			return fmt.Errorf("Task %q (%.20q) Could not convert due time to gcal event time: %w", task.UUID, task.Description, err)
		}

		var action bool

		if task.CalendarID != "" {
			event, err := getEvent(srv, task.CalendarID)
			if err != nil {
				fmt.Printf("Could not retrieve event for calendar ID %q: %v\n", task.CalendarID, err)
				continue
			}

			m, err := unify(task, event)
			if err != nil {
				return fmt.Errorf("Error reconciling task and event: %w", err)
			}

			if m {
				event, err = modifyEvent(srv, event)
				if err != nil {
					return fmt.Errorf("Error modifying calendar event: %w", err)
				}
				action = true
			}
		} else {
			var err error

			event, err := insertEvent(srv, &calendar.Event{
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
			resyncTasks = append(resyncTasks, task)
		}
	}

	for _, task := range unsyncedTasks {
		due, err := taskTimeToGCal(task.Due)
		if err != nil {
			return fmt.Errorf("Task %q (%.20q) Could not convert due time to gcal event time: %w", task.UUID, task.Description, err)
		}

		event, err := insertEvent(srv, &calendar.Event{
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
			resyncTasks = append(resyncTasks, task)
		}
	}

	for _, task := range deletedTasks {
		if err := deleteEvent(srv, task.CalendarID); err == nil {
			fmt.Printf("Deleting task %q (%.20q)\n", task.UUID, task.Description)
		}
	}

	if err := tw.importTasks(resyncTasks); err != nil {
		return fmt.Errorf("Could not import tasks: %w", err)
	}

	return nil
}
