package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"google.golang.org/api/calendar/v3"
)

const twTimeFormat = "20060102T150405Z"

type taskWarrior struct {
	path string
}

func findTaskwarrior() (*taskWarrior, error) {
	for _, path := range []string{"task", "taskw"} {
		s, err := exec.LookPath(path)
		if err == nil {
			return &taskWarrior{path: s}, nil
		}
	}

	return nil, errors.New("Could not find taskwarrior")
}

func (tw *taskWarrior) runTask(args ...string) ([]byte, error) {
	return exec.Command(tw.path, args...).Output()
}

// runs the command and attempts to read the tasks. `export` argument
// is required in your args stanza.
// f.e.: `export all`
func (tw *taskWarrior) exportTasksByCommand(args ...string) (taskWarriorItems, error) {
	out, err := tw.runTask(args...)
	if err != nil {
		return nil, fmt.Errorf("Could not export tasks: %w", err)
	}

	items := taskWarriorItems{}
	return items, items.unmarshalItems(out)
}

func (tw *taskWarrior) importTasks(items taskWarriorItems) error {
	cmd := exec.Command(tw.path, "import")

	pipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("Trouble establishing stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Could not start 'task import': %w", err)
	}
	defer cmd.Process.Kill()

	out, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("Could not marshal items: %w", err)
	}

	if _, err := pipe.Write(out); err != nil {
		return fmt.Errorf("Could not write json to pipe: %w", err)
	}
	pipe.Close()

	return cmd.Wait()
}

type taskWarriorTime string

func (twt taskWarriorTime) ToTime() (time.Time, error) {
	if twt == "" {
		return time.Time{}, errors.New("time is blank")
	}
	return time.Parse(twTimeFormat, string(twt))
}

func (twt taskWarriorTime) ToGCal() (*calendar.EventDateTime, error) {
	t, err := twt.ToTime()
	if err != nil {
		return nil, err
	}

	return &calendar.EventDateTime{DateTime: string(toCalendarTime(t)), TimeZone: time.Local.String()}, nil
}

func toTaskWarriorTime(t time.Time) taskWarriorTime {
	return taskWarriorTime(t.Format(twTimeFormat))
}

type taskWarriorItems []*taskWarriorItem

type taskWarriorItem struct {
	ID          int             `json:"id"`
	Status      string          `json:"status"`
	UUID        string          `json:"uuid"`
	Entry       taskWarriorTime `json:"entry"`
	Description string          `json:"description"`
	Start       taskWarriorTime `json:"start,omitempty"`
	End         taskWarriorTime `json:"end,omitempty"`
	Due         taskWarriorTime `json:"due,omitempty"`
	Until       taskWarriorTime `json:"until,omitempty"`
	Wait        taskWarriorTime `json:"wait,omitempty"`
	Modified    taskWarriorTime `json:"modified,omitempty"`
	Scheduled   taskWarriorTime `json:"scheduled,omitempty"`
	Recur       string          `json:"recur,omitempty"`
	Mask        string          `json:"mask,omitempty"`
	IMask       float64         `json:"imask,omitempty"`
	Parent      string          `json:"parent,omitempty"`
	Project     string          `json:"project,omitempty"`
	Priority    string          `json:"priority,omitempty"`
	Depends     string          `json:"depends,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	Annotation  []string        `json:"annotation,omitempty"`
	CalendarID  string          `json:"udf.calwarrior.id,omitempty"`
}

func (twi *taskWarriorItems) unmarshalItems(out []byte) error {
	if err := json.Unmarshal(out, twi); err != nil {
		return fmt.Errorf("Could not parse JSON: %w", err)
	}

	return nil
}

type taskWarriorTags []string

func (twt taskWarriorTags) decorate() []string {
	tags := []string{}

	for i := range twt {
		tags = append(tags, "+"+twt[i])
	}

	return tags
}
