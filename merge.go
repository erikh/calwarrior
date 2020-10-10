package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/api/calendar/v3"
)

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

	ct, err := calendarTime(event.Updated).ToTaskWarriorTime()
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
			event.Start, err = task.Due.ToGCal()
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

type merge struct {
	tw  *taskWarrior
	cal *calendarClient
	ctx *cliContext
	log logLevel

	unsyncedEvents    []*calendar.Event
	eventsIDMap       map[string]*calendar.Event
	unsyncedTasks     taskWarriorItems
	checkTasks        taskWarriorItems
	deletedTasks      taskWarriorItems
	deletedTaskCalMap map[string]*taskWarriorItem
	taskIDMap         map[string]*taskWarriorItem
}

func newMerge(ctx *cliContext, tw *taskWarrior, cal *calendarClient) *merge {
	return &merge{
		tw:  tw,
		cal: cal,
		ctx: ctx,
		log: os.Getenv("DEBUG") != "",

		unsyncedEvents:    []*calendar.Event{},
		eventsIDMap:       map[string]*calendar.Event{},
		unsyncedTasks:     taskWarriorItems{},
		checkTasks:        taskWarriorItems{},
		deletedTasks:      taskWarriorItems{},
		deletedTaskCalMap: map[string]*taskWarriorItem{},
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
				m.deletedTaskCalMap[task.CalendarID] = task
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
		task, ok := m.taskIDMap[event.Id]
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
		if task, ok := m.deletedTaskCalMap[event.Id]; ok {
			m.log.DeleteTask(task)
			continue
		}

		m.log.SyncTask(event)
		due, err := eventDue(event)
		if err != nil {
			return err
		}

		m.checkTasks = append(m.checkTasks, &taskWarriorItem{
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
		m.log.Warnf("Could not retrieve event for calendar ID (already removed?) %q: %v", task.CalendarID, err)
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

func (m *merge) filterTasks() {
	tm := map[string]*taskWarriorItem{} // UUID -> item

	newTasks := taskWarriorItems{}

	// overwrite in order
	for _, task := range m.checkTasks {
		// skip if no UUID
		if task.UUID == "" {
			m.log.AddTask(task)
			newTasks = append(newTasks, task)
			continue
		}

		if _, ok := tm[task.UUID]; ok {
			fmt.Printf("Filtering duplicate task %q (%q)\n", task.UUID, task.Description)
		}

		tm[task.UUID] = task
	}

	for _, task := range tm {
		newTasks = append(newTasks, task)
	}

	m.checkTasks = newTasks
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
			due, err := task.Due.ToGCal()
			if err != nil {
				return fmt.Errorf("Task %q (%.20q) Could not convert due time to gcal event time: %w", task.UUID, task.Description, err)
			}

			m.log.SyncEvent(task)
			action, err = m.createNewEvent(task, due)
			if err != nil {
				return err
			}
		}

		if action {
			m.checkTasks = append(m.checkTasks, task)
		}
	}

	for _, task := range m.unsyncedTasks {
		due, err := task.Due.ToGCal()
		if err != nil {
			return fmt.Errorf("Task %q (%.20q) Could not convert due time to gcal event time: %w", task.UUID, task.Description, err)
		}

		modified, err := m.createNewEvent(task, due)
		if err != nil {
			return err
		}

		if modified {
			m.checkTasks = append(m.checkTasks, task)
		}
	}

	for _, task := range m.deletedTasks {
		if err := m.cal.deleteEvent(task.CalendarID); err == nil {
			m.log.DeleteEvent(m.eventsIDMap[task.CalendarID])
		} else {
			m.log.Error(err)
		}
	}

	m.filterTasks()

	if err := m.tw.importTasks(m.checkTasks); err != nil {
		return fmt.Errorf("Could not import tasks: %w", err)
	}

	return nil
}
