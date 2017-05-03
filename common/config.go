// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package common

// Command line options.
const (
	// Operating environment.
	OptEnvironment      = "environment"
	OptEnvironmentAlias = "e"
	OptEnvironmentAzure = "azure"
	OptEnvironmentMAS   = "mas"

	// API server URL.
	OptAPIServerURL      = "api-url"
	OptAPIServerURLAlias = "u"

	// Logging level.
	OptLogLevel      = "log-level"
	OptLogLevelAlias = "l"
	OptLogLevelInfo  = "info"
	OptLogLevelDebug = "debug"

	// Logging target.
	OptLogTarget       = "log-target"
	OptLogTargetAlias  = "t"
	OptLogTargetSyslog = "syslog"
	OptLogTargetStderr = "stderr"
	OptLogTargetFile   = "logfile"

	// IPAM query URL.
	OptIpamQueryUrl      = "ipam-query-url"
	OptIpamQueryUrlAlias = "q"

	// IPAM query interval.
	OptIpamQueryInterval      = "ipam-query-interval"
	OptIpamQueryIntervalAlias = "i"

	// Version.
	OptVersion      = "version"
	OptVersionAlias = "v"

	// Help.
	OptHelp      = "help"
	OptHelpAlias = "h"
)
