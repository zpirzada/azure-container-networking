// Copyright Microsoft Corp.
// All rights reserved.

package log

import (
	"log"
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
	TargetLogfile
)

// Log prefix
const logPrefix = ""

// Logger object
type Logger struct {
	l     *log.Logger
	name  string
	level int
}

// NewLogger creates a new Logger.
func NewLogger(name string, level int, target int) *Logger {
	var logger Logger

	logger.l = log.New(nil, logPrefix, log.LstdFlags)
	logger.name = name
	logger.level = level
	logger.SetTarget(target)

	return &logger
}

// SetName sets the log name.
func (logger *Logger) SetName(name string) {
	logger.name = name
}

// SetLevel sets the log chattiness.
func (logger *Logger) SetLevel(level int) {
	logger.level = level
}

// Request logs a structured request.
func (logger *Logger) Request(tag string, request interface{}, err error) {
	if err == nil {
		logger.Printf("[%s] Received %T %+v.", tag, request, request)
	} else {
		logger.Printf("[%s] Failed to decode %T %+v %s.", tag, request, request, err.Error())
	}
}

// Response logs a structured response.
func (logger *Logger) Response(tag string, response interface{}, err error) {
	if err == nil {
		logger.Printf("[%s] Sent %T %+v.", tag, response, response)
	} else {
		logger.Printf("[%s] Failed to encode %T %+v %s.", tag, response, response, err.Error())
	}
}

// Printf logs a formatted string at info level.
func (logger *Logger) Printf(format string, args ...interface{}) {
	if logger.level >= LevelInfo {
		logger.l.Printf(format, args...)
	}
}

// Debugf logs a formatted string at debug level.
func (logger *Logger) Debugf(format string, args ...interface{}) {
	if logger.level >= LevelDebug {
		logger.l.Printf(format, args...)
	}
}
