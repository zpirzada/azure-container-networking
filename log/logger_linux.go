// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package log

import (
	"fmt"
	"io"
	"log"
	"log/syslog"
	"os"

	"github.com/Azure/azure-container-networking/platform"
)

// Log file properties.
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
		fileName := platform.LogPath + logger.name + logFileExtension
		out, err = os.OpenFile(fileName, os.O_CREATE|os.O_APPEND|os.O_RDWR, logFilePerm)
	default:
		err = fmt.Errorf("Invalid log target %d", target)
	}

	if err == nil {
		logger.l.SetOutput(out)
	}

	return err
}
