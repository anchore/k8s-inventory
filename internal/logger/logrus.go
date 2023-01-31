// Kai's Logging implementation via Logrus
package logger

import (
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

const defaultLogPermissions fs.FileMode = 0644

// LogrusConfiguration for Kai
type LogrusConfig struct {
	EnableConsole bool
	EnableFile    bool
	Structured    bool
	Level         logrus.Level
	FileLocation  string
}

// Wraps the internal Logrus implementation
type LogrusLogger struct {
	Config LogrusConfig
	Logger *logrus.Logger
}

// Nested Logging
type LogrusNestedLogger struct {
	Logger *logrus.Entry
}

// Constructor for Logrus Logger (initialized in cmd)
func NewLogrusLogger(cfg LogrusConfig) *LogrusLogger {
	appLogger := logrus.New()

	var output io.Writer
	switch {
	case cfg.EnableConsole && cfg.EnableFile:
		logFile, err := os.OpenFile(cfg.FileLocation, os.O_APPEND|os.O_CREATE|os.O_WRONLY, defaultLogPermissions)
		if err != nil {
			panic(fmt.Errorf("unable to setup log file: %w", err))
		}
		output = io.MultiWriter(os.Stderr, logFile)
	case cfg.EnableConsole:
		output = os.Stderr
	case cfg.EnableFile:
		logFile, err := os.OpenFile(cfg.FileLocation, os.O_APPEND|os.O_CREATE|os.O_WRONLY, defaultLogPermissions)
		if err != nil {
			panic(fmt.Errorf("unable to setup log file: %w", err))
		}
		output = logFile
	default:
		output = io.Discard
	}

	appLogger.SetOutput(output)
	appLogger.SetLevel(cfg.Level)

	if cfg.Structured {
		appLogger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat:   "2006-01-02 15:04:05",
			DisableTimestamp:  false,
			DisableHTMLEscape: false,
			PrettyPrint:       false,
		})
	} else {
		appLogger.SetFormatter(&prefixed.TextFormatter{
			DisableColors:   true,
			TimestampFormat: "2006-01-02 15:04:05",
			FullTimestamp:   true,
			ForceFormatting: true,
		})
	}

	return &LogrusLogger{
		Config: cfg,
		Logger: appLogger,
	}
}

func (l *LogrusLogger) Debugf(format string, args ...interface{}) {
	l.Logger.Debugf(format, args...)
}

func (l *LogrusLogger) Infof(format string, args ...interface{}) {
	l.Logger.Infof(format, args...)
}

func (l *LogrusLogger) Debug(args ...interface{}) {
	l.Logger.Debug(args...)
}

func (l *LogrusLogger) Info(args ...interface{}) {
	l.Logger.Info(args...)
}

func (l *LogrusLogger) Warnf(format string, args ...interface{}) {
	l.Logger.Warnf(format, args...)
}

func (l *LogrusLogger) Errorf(format string, args ...interface{}) {
	l.Logger.Errorf(format, args...)
}

func (l *LogrusNestedLogger) Debugf(format string, args ...interface{}) {
	l.Logger.Debugf(format, args...)
}

func (l *LogrusNestedLogger) Infof(format string, args ...interface{}) {
	l.Logger.Infof(format, args...)
}

func (l *LogrusNestedLogger) Debug(args ...interface{}) {
	l.Logger.Debug(args...)
}

func (l *LogrusNestedLogger) Info(args ...interface{}) {
	l.Logger.Info(args...)
}

func (l *LogrusNestedLogger) Warnf(format string, args ...interface{}) {
	l.Logger.Warnf(format, args...)
}

func (l *LogrusNestedLogger) Errorf(format string, args ...interface{}) {
	l.Logger.Errorf(format, args...)
}
