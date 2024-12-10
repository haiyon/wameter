package config

import "wameter/internal/logger"

// LogConfig represents logging configuration
// This is a copy of the logger.Config
type LogConfig = logger.Config

var (
	// AppName is the name of the application
	AppName = "wameter"

	// Config search paths

	// InDot is the path to the config file in ./
	InDot = "."
	// InEtc is the path to the config file in /etc/{AppName}
	InEtc = "/etc/" + AppName
	// InHome is the path to the config file in $HOME/.config/{AppName}
	InHome = "$HOME/.config/" + AppName
	// InHomeDot is the path to the config file in $HOME/.{AppName}
	InHomeDot = "$HOME/." + AppName
)
