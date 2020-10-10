package main

import (
	"fmt"

	"github.com/fatih/color"
	"google.golang.org/api/calendar/v3"
)

type logLevel bool // is debugging on or not

type logger interface {
	AddTask(*taskWarriorItem)
	AddEvent(*calendar.Event)
	SyncTask(*calendar.Event)
	SyncEvent(*taskWarriorItem)
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
	printfln(color.HiYellowString(s), args...)
}

func (l logLevel) Warnf(s string, args ...interface{}) {
	printfln(color.HiWhiteString(s), args...)
}

func (l logLevel) Error(err error) {
	printfln(color.RedString("%v"), err)
}

func (l logLevel) AddTask(task *taskWarriorItem) {
	printfln(color.BlueString("Creating new task for %q"), task.Description)
}

func (l logLevel) AddEvent(event *calendar.Event) {
	printfln(color.CyanString("Creating new calendar event for %q"), event.Summary)
}

func (l logLevel) SyncTask(event *calendar.Event) {
	printfln(color.HiBlueString("Syncing %q (%.20q) to taskwarrior"), event.Id, event.Summary)
}

func (l logLevel) SyncEvent(task *taskWarriorItem) {
	printfln(color.HiCyanString("Pushing %q (%.20q) to gcal"), task.UUID, task.Description)
}

func (l logLevel) DeleteTask(task *taskWarriorItem) {
	printfln(color.HiMagentaString("Deleting task %q (%.20q)"), task.UUID, task.Description)
}

func (l logLevel) DeleteEvent(event *calendar.Event) {
	printfln(color.HiMagentaString("Deleting event %q (%.20q)"), event.Id, event.Summary)
}
