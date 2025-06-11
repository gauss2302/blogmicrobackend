package logger

import (
	"log"
	"os"
	"strings"
)

type Logger struct {
	*log.Logger
	level LogLevel
}

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

func New(level string) *Logger {
	return &Logger{
		Logger: log.New(os.Stdout, "[API-GATEWAY] ", log.LstdFlags|log.Lshortfile),
		level:  parseLogLevel(level),
	}
}

func (l *Logger) Debug(msg string) {
	if l.level <= DEBUG {
		l.Printf("[DEBUG] %s", msg)
	}
}

func (l *Logger) Info(msg string) {
	if l.level <= INFO {
		l.Printf("[INFO] %s", msg)
	}
}

func (l *Logger) Warn(msg string) {
	if l.level <= WARN {
		l.Printf("[WARN] %s", msg)
	}
}

func (l *Logger) Error(msg string) {
	if l.level <= ERROR {
		l.Printf("[ERROR] %s", msg)
	}
}

func (l *Logger) Fatal(msg string) {
	l.Printf("[FATAL] %s", msg)
	os.Exit(1)
}

func parseLogLevel(level string) LogLevel {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN":
		return WARN
	case "ERROR":
		return ERROR
	case "FATAL":
		return FATAL
	default:
		return INFO
	}
}