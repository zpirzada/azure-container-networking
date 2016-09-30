// Copyright Microsoft Corp.
// All rights reserved.

package common

// Command line options.
const (
	// Operating environment.
	OptEnvironmentKey      = "environment"
	OptEnvironmentKeyShort = "e"
	OptEnvironmentAzure    = "azure"
	OptEnvironmentMAS      = "mas"

	// Logging level.
	OptLogLevelKey      = "log-level"
	OptLogLevelKeyShort = "l"
	OptLogLevelInfo     = "info"
	OptLogLevelDebug    = "debug"

	// Logging target.
	OptLogTargetKey      = "log-target"
	OptLogTargetKeyShort = "t"
	OptLogTargetSyslog   = "syslog"
	OptLogTargetStderr   = "stderr"

	// Help.
	OptHelpKey      = "help"
	OptHelpKeyShort = "?"
)
