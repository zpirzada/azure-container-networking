// Copyright Microsoft Corp.
// All rights reserved.

package log

// Standard logger is a pre-defined logger for convenience.
var stdLog *Logger = NewLogger()

// Helper functions for the standard logger.
func GetStd() *Logger {
	return stdLog
}

func SetTarget(target int) error {
	return stdLog.SetTarget(target)
}

func SetLevel(level int) {
	stdLog.SetLevel(level)
}

func Request(tag string, requestName string, request interface{}, err error) {
	stdLog.Request(tag, requestName, request, err)
}

func Response(tag string, responseName string, response interface{}, err error) {
	stdLog.Response(tag, responseName, response, err)
}

func Printf(format string, args ...interface{}) {
	stdLog.Printf(format, args)
}

func Println(s string) {
	stdLog.Println(s)
}
