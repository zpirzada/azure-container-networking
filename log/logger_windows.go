// Copyright Microsoft Corp.
// All rights reserved.

package log

import (
	"fmt"
	"io"
	"os"
)

// Log file properties.
const logFilePath = ""
const logFilePerm = os.FileMode(0664)

// SetTarget sets the log target.
func (logger *Logger) SetTarget(target int) error {
	var out io.Writer
	var err error

	switch target {
	case TargetStderr:
		out = os.Stderr
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
