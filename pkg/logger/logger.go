package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// LogLevel represents the logging level
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

var (
	levelFlags = []string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}
	levelMap   = map[string]LogLevel{
		"debug": LevelDebug,
		"info":  LevelInfo,
		"warn":  LevelWarn,
		"error": LevelError,
		"fatal": LevelFatal,
	}
)

// Logger interface
type Logger interface {
	Debug(format string, v ...interface{})
	Info(format string, v ...interface{})
	Warn(format string, v ...interface{})
	Error(format string, v ...interface{})
	Fatal(format string, v ...interface{})
}

// logger implementation
type logger struct {
	level LogLevel
	mu    sync.Mutex
}

var (
	instance Logger
	once     sync.Once
)

// GetLogger returns a singleton logger instance
func GetLogger() Logger {
	once.Do(func() {
		levelStr := strings.ToLower(os.Getenv("LOG_LEVEL"))
		if levelStr == "" {
			levelStr = "info"
		}

		level, ok := levelMap[levelStr]
		if !ok {
			level = LevelInfo
		}

		instance = &logger{level: level}
	})
	return instance
}

// SetLogLevel sets the log level
func SetLogLevel(level string) {
	if l, ok := levelMap[strings.ToLower(level)]; ok {
		if lg, ok := instance.(*logger); ok {
			lg.level = l
		}
	}
}

func (l *logger) log(level LogLevel, format string, v ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	prefix := levelFlags[level]
	timestamp := getTimestamp()
	msg := fmt.Sprintf(format, v...)

	logStr := fmt.Sprintf("[%s][%s]%s", timestamp, prefix, msg)

	switch level {
	case LevelFatal:
		log.Fatal(logStr)
	default:
		log.Println(logStr)
	}
}

func (l *logger) Debug(format string, v ...interface{}) {
	l.log(LevelDebug, format, v...)
}

func (l *logger) Info(format string, v ...interface{}) {
	l.log(LevelInfo, format, v...)
}

func (l *logger) Warn(format string, v ...interface{}) {
	l.log(LevelWarn, format, v...)
}

func (l *logger) Error(format string, v ...interface{}) {
	l.log(LevelError, format, v...)
}

func (l *logger) Fatal(format string, v ...interface{}) {
	l.log(LevelFatal, format, v...)
}

func getTimestamp() string {
	return time.Now().Format("2006-01-02 15:04:05")
}