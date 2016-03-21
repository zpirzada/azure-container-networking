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
func (logger *Logger) Request(tag string, requestName string, request interface{}, err error) {
	if err == nil {
		logger.l.Printf("%s: Received %s request %+v.", tag, requestName, request)
	} else {
		logger.l.Printf("%s: Failed to decode %s request %+v %s.", tag, requestName, request, err.Error())
	}
}

// Logs a structured response.
func (logger *Logger) Response(tag string, responseName string, response interface{}, err error) {
	if err == nil {
		logger.l.Printf("%s: Sent %s response %+v.", tag, responseName, response)
	} else {
		logger.l.Printf("%s: Failed to encode %s response %+v %s.", tag, responseName, response, err.Error())
	}
}

// Logs a formatted string.
func (logger *Logger) Printf(format string, args ...interface{}) {
	logger.l.Printf(format, args)
}
