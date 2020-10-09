package main

import (
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

func (m *merge) makeIDMaps(tasks taskWarriorItems, events *calendar.Events) {
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
}

func (m *merge) determineAlreadySyncedTaskWarrior(events *calendar.Events) error {
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

	return nil
}

func (m *merge) syncTasks() error {
	tags, err := m.ctx.makeTags()
	if err != nil {
		return err
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

	return nil
}

func (m *merge) get() (taskWarriorItems, *calendar.Events, error) {
	tags, err := m.ctx.makeTags()
	if err != nil {
		return nil, nil, err
	}

	tasks, err := m.tw.exportTasksByCommand(append([]string{"export"}, tags.decorate()...)...)
	if err != nil {
		return nil, nil, err
	}

	t1, t2 := m.ctx.getTimeWindow()

	events, err := m.cal.gatherEvents(t1, t2)
	if err != nil {
		return nil, nil, fmt.Errorf("Trouble gathering events: %w", err)
	}

	return tasks, events, nil
}

func (m *merge) mergeExistingEvent(task *taskWarriorItem) (bool, error) {
	event, err := m.cal.getEvent(task.CalendarID)
	if err != nil {
		fmt.Printf("Could not retrieve event for calendar ID (already removed?) %q: %v\n", task.CalendarID, err)
		return false, nil
	}

	modified, err := unify(task, event)
	if err != nil {
		return false, fmt.Errorf("Error reconciling task and event: %w", err)
	}

	if modified {
		event, err = m.cal.modifyEvent(event)
		if err != nil {
			return false, fmt.Errorf("Error modifying calendar event: %w", err)
		}
	}
	return modified, nil
}

func (m *merge) createNewEvent(task *taskWarriorItem, due *calendar.EventDateTime) (bool, error) {
	var err error

	event, err := m.cal.insertEvent(&calendar.Event{
		Summary: task.Description,
		Start:   due,
		End:     due,
	})
	if err != nil {
		return false, fmt.Errorf("Error inserting calendar event: %w", err)
	}

	modified, err := unify(task, event)
	if err != nil {
		return false, fmt.Errorf("Error reconciling task and event: %w", err)
	}

	return modified, nil
}

func (m *merge) run() error {
	tasks, events, err := m.get()
	if err != nil {
		return err
	}

	m.makeIDMaps(tasks, events)

	if err := m.determineAlreadySyncedTaskWarrior(events); err != nil {
		return err
	}

	if err := m.syncTasks(); err != nil {
		return err
	}

	for _, task := range m.checkTasks {
		var action bool

		if task.CalendarID != "" {
			action, err = m.mergeExistingEvent(task)
			if err != nil {
				return err
			}
		} else {
			due, err := taskTimeToGCal(task.Due)
			if err != nil {
				return fmt.Errorf("Task %q (%.20q) Could not convert due time to gcal event time: %w", task.UUID, task.Description, err)
			}

			action, err = m.createNewEvent(task, due)
			if err != nil {
				return err
			}
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

		modified, err := m.createNewEvent(task, due)
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
