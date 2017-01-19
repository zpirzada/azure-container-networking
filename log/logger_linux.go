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

// Log file properties.
const logFilePath = "/var/log/"
const logFilePerm = os.FileMode(0664)

const syslogTag = "AzureContainerNet"

// SetTarget sets the log target.
func (logger *Logger) SetTarget(target int) error {
	var out io.Writer
	var err error

	switch target {
	case TargetStderr:
		out = os.Stderr
	case TargetSyslog:
		out, err = syslog.New(log.LstdFlags, syslogTag)
	case TargetLogfile:
		fileName := logFilePath + logger.name + ".log"
		out, err = os.OpenFile(fileName, os.O_CREATE|os.O_APPEND|os.O_RDWR, logFilePerm)
	default:
		err = fmt.Errorf("Invalid log target %d", target)
	}

	if err == nil {
		logger.l.SetOutput(out)
	}

	return err
}
