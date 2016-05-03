// Copyright Microsoft Corp.
// All rights reserved.

package log

import (
	"fmt"
	"io"
	"log"
	"log/syslog"
	"os"
)

// Log level
const (
	LevelAlert = iota
	LevelError
	LevelWarning
	LevelInfo
	LevelDebug
)

// Log target
const (
	TargetStderr = iota
	TargetSyslog
	TargetFile
)

// Log prefix
const logPrefix = ""
const syslogTag = "AQUA"

// Logger object
type Logger struct {
	l     *log.Logger
	level int
}

// Creates a new Logger with default settings.
func NewLogger() *Logger {
	var logger Logger

	logger.l = log.New(os.Stderr, logPrefix, log.LstdFlags)
	logger.level = LevelInfo

	return &logger
}

// Sets the log target.
func (logger *Logger) SetTarget(target int) error {
	var out io.Writer
	var err error

	switch target {
	case TargetStderr:
		out = os.Stderr
	case TargetSyslog:
		out, err = syslog.New(log.LstdFlags, syslogTag)
	default:
		err = fmt.Errorf("Invalid log target %d", target)
	}

	if err == nil {
		logger.l.SetOutput(out)
	}

	return err
}

// Sets the log chattiness.
func (logger *Logger) SetLevel(level int) {
	logger.level = level
}

// Logs a structured request.
func (logger *Logger) Request(tag string, request interface{}, err error) {
	if err == nil {
		logger.l.Printf("%s: Received %T %+v.", tag, request, request)
	} else {
		logger.l.Printf("%s: Failed to decode %T %+v %s.", tag, request, request, err.Error())
	}
}

// Logs a structured response.
func (logger *Logger) Response(tag string, response interface{}, err error) {
	if err == nil {
		logger.l.Printf("%s: Sent %T %+v.", tag, response, response)
	} else {
		logger.l.Printf("%s: Failed to encode %T %+v %s.", tag, response, response, err.Error())
	}
}

// Logs a formatted string.
func (logger *Logger) Printf(format string, args ...interface{}) {
	logger.l.Printf(format, args...)
}

// Logs a string.
func (logger *Logger) Println(s string) {
	logger.l.Println(s)
}
