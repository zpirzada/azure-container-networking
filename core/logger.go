// Copyright Microsoft Corp.
// All rights reserved.

package core

import (
	"fmt"
	"io"
	"log"
	"log/syslog"
	"os"
)

// Log severity
const (
	LOG_ALT = iota
	LOG_ERR
	LOG_WRN
	LOG_INF
	LOG_DBG
)

// Log target
const (
	LOG_STDERR = iota
	LOG_SYSLOG
	LOG_FILE
)

// Log prefix
const logPrefix = ""
const syslogTag = "PENGUIN"

// Logger object
type Logger struct {
    l *log.Logger
}

// Creates a new Logger.
func NewLogger(target int) (*Logger, error) {
	var logger Logger
	var out io.Writer
	var err error

	switch target {
	case LOG_STDERR:
		out = os.Stderr
	case LOG_SYSLOG:
		out, err = syslog.New(log.LstdFlags, syslogTag)
	default:
		err = fmt.Errorf("Invalid log target %d", target)
	}

    if err == nil {
		logger.l = log.New(out, logPrefix, log.LstdFlags)
	}

	return &logger, err
}

func (logger *Logger) LogRequest(tag string, requestName string, err error) {
	if err == nil {
		logger.l.Printf("%s: Received %s request.", tag, requestName)
	} else {
		logger.l.Printf("%s: Failed to decode %s request %s.", tag, requestName, err.Error())
	}
}

func (logger *Logger) LogResponse(tag string, responseName string, response interface{}, err error) {
	if err == nil {
		logger.l.Printf("%s: Sent %s response %+v.", tag, responseName, response)
	} else {
		logger.l.Printf("%s: Failed to encode %s response %+v %s.", tag, responseName, response, err.Error())
	}
}

func (logger *Logger) LogEvent(tag string, event string) {
	logger.l.Printf("%s: %s", tag, event)
}
