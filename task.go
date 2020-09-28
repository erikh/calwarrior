package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

const twTimeFormat = "20060102T150405Z"

type taskWarriorTime string

func (twt taskWarriorTime) ToTime() (time.Time, error) {
	if twt == "" {
		return time.Time{}, errors.New("time is blank")
	}
	return time.Parse(twTimeFormat, string(twt))
}

func toTaskWarriorTime(t time.Time) taskWarriorTime {
	return taskWarriorTime(t.Format(twTimeFormat))
}

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

func findTaskwarrior() (string, error) {
	for _, path := range []string{"task", "taskw"} {
		s, err := exec.LookPath(path)
		if err == nil {
			return s, nil
		}
	}

	return "", errors.New("Could not find taskwarrior")
}

func runTask(tw string, args ...string) ([]byte, error) {
	return exec.Command(tw, args...).Output()
}

func unmarshalItems(out []byte) ([]*taskWarriorItem, error) {
	items := []*taskWarriorItem{}

	if err := json.Unmarshal(out, &items); err != nil {
		return nil, fmt.Errorf("Could not parse JSON: %w", err)
	}

	return items, nil
}

func exportTasks(tw string, uuids ...string) ([]*taskWarriorItem, error) {
	out, err := runTask(tw, append([]string{"export"}, uuids...)...)
	if err != nil {
		return nil, fmt.Errorf("Could not export tasks: %w", err)
	}

	return unmarshalItems(out)
}

// this one runs the command and attempts to read the tasks. `export` argument
// is required in your args stanza.
// f.e.: `export all`
func exportTasksByCommand(tw string, args ...string) ([]*taskWarriorItem, error) {
	out, err := runTask(tw, args...)
	if err != nil {
		return nil, fmt.Errorf("Could not export tasks: %w", err)
	}

	return unmarshalItems(out)
}

func importTasks(tw string, items []*taskWarriorItem) error {
	cmd := exec.Command(tw, "import")

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
