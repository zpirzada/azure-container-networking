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
	TargetLogfile
)

// Log path and file
const logFile = "/var/log/azure-container-networking.log"
const logFilePerm = os.FileMode(0664)

// Log prefix
const logPrefix = ""
const syslogTag = "AzureContainerNet"

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
	case TargetLogfile:
		out, err = os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_RDWR, logFilePerm)
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
		logger.Printf("[%s] Received %T %+v.", tag, request, request)
	} else {
		logger.Printf("[%s] Failed to decode %T %+v %s.", tag, request, request, err.Error())
	}
}

// Logs a structured response.
func (logger *Logger) Response(tag string, response interface{}, err error) {
	if err == nil {
		logger.Printf("[%s] Sent %T %+v.", tag, response, response)
	} else {
		logger.Printf("[%s] Failed to encode %T %+v %s.", tag, response, response, err.Error())
	}
}

// Logs a formatted string at info level.
func (logger *Logger) Printf(format string, args ...interface{}) {
	if logger.level >= LevelInfo {
		logger.l.Printf(format, args...)
	}
}

// Logs a formatted string at debug level.
func (logger *Logger) Debugf(format string, args ...interface{}) {
	if logger.level >= LevelDebug {
		logger.l.Printf(format, args...)
	}
}
