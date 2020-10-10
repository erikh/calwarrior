package main

import (
	"fmt"

	"google.golang.org/api/calendar/v3"
)

type logLevel bool // is debugging on or not

type logger interface {
	AddTask(*taskWarriorItem)
	AddEvent(*calendar.Event)
	SyncTask(*calendar.Event)
	DeleteTask(*taskWarriorItem)
	DeleteEvent(*calendar.Event)
	Error(error)
	Noticef(string, ...interface{})
	Warnf(string, ...interface{})
	Debugf(string, ...interface{})
}

func printfln(s string, args ...interface{}) {
	fmt.Printf(s+"\n", args...)
}

func (l logLevel) Debugf(s string, args ...interface{}) {
	if l {
		printfln(s, args...)
	}
}

func (l logLevel) Noticef(s string, args ...interface{}) {
	printfln(s, args...)
}

func (l logLevel) Warnf(s string, args ...interface{}) {
	printfln(s, args...)
}

func (l logLevel) Error(err error) {
	printfln("%v", err)
}

func (l logLevel) AddTask(task *taskWarriorItem) {
	printfln("Creating new task for %q", task.Description)
}

func (l logLevel) AddEvent(event *calendar.Event) {
	printfln("Creating new task for %q", event.Summary)
}

func (l logLevel) SyncTask(event *calendar.Event) {
	printfln("Syncing %q (%.20q) to taskwarrior", event.Id, event.Summary)
}

func (l logLevel) SyncEvent(task *taskWarriorItem) {
	printfln("Pushing %q (%.20q) to gcal", task.UUID, task.Description)
}

func (l logLevel) DeleteTask(task *taskWarriorItem) {
	printfln("Deleting task %q (%.20q)", task.UUID, task.Description)
}

func (l logLevel) DeleteEvent(event *calendar.Event) {
	printfln("Deleting event %q (%.20q)", event.Id, event.Summary)
}
