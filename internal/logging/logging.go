package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/devsync/dev-sync/internal/config"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "INFO"
	}
}

// Logger handles rotated file logging and terminal formatting.
type Logger struct {
	mu           sync.Mutex
	logDir       string
	activeFile   *os.File
	level        Level
	noColor      bool
	quiet        bool
	maxSizeBytes int64
}

var globalLogger *Logger
var once sync.Once

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

// Init initializes the global logger for the given project root.
func Init(projectRoot string, level Level, quiet bool) (*Logger, error) {
	if globalLogger != nil {
		globalLogger.mu.Lock()
		if globalLogger.activeFile != nil {
			_ = globalLogger.activeFile.Close()
			globalLogger.activeFile = nil
		}
		globalLogger.mu.Unlock()
	}

	logDir := filepath.Join(projectRoot, config.ConfigDirName, config.LogDirName)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	archiveDir := filepath.Join(logDir, "archive")
	_ = os.MkdirAll(archiveDir, 0755)

	noColor := false
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		noColor = true
	}

	l := &Logger{
		logDir:       logDir,
		level:        level,
		noColor:      noColor,
		quiet:        quiet,
		maxSizeBytes: 5 * 1024 * 1024, // 5MB
	}

	if err := l.openLogFile(); err != nil {
		return nil, err
	}

	globalLogger = l
	return l, nil
}

// Get returns the global logger or a fallback stderr logger if uninitialized.
func Get() *Logger {
	once.Do(func() {
		if globalLogger == nil {
			globalLogger = &Logger{
				level:   LevelInfo,
				noColor: true,
			}
		}
	})
	return globalLogger
}

func (l *Logger) openLogFile() error {
	logPath := filepath.Join(l.logDir, "dev-sync.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	l.activeFile = f
	return nil
}

func (l *Logger) checkRotate() {
	if l.activeFile == nil {
		return
	}
	fi, err := l.activeFile.Stat()
	if err != nil || fi.Size() < l.maxSizeBytes {
		return
	}

	_ = l.activeFile.Close()

	archiveDir := filepath.Join(l.logDir, "archive")
	archivePath := filepath.Join(archiveDir, fmt.Sprintf("dev-sync-%s.log", time.Now().Format("20060102-150405")))
	logPath := filepath.Join(l.logDir, "dev-sync.log")

	_ = os.Rename(logPath, archivePath)
	_ = l.openLogFile()
}

func (l *Logger) logMessage(lvl Level, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	msg := fmt.Sprintf(format, args...)
	// Strip any webhook secrets just in case
	msg = config.MaskWebhookURL(msg)

	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	fileLine := fmt.Sprintf("[%s] [%s] %s\n", timestamp, lvl.String(), msg)

	if l.activeFile != nil {
		l.checkRotate()
		_, _ = io.WriteString(l.activeFile, fileLine)
	}

	if !l.quiet && lvl >= l.level {
		l.printConsole(lvl, msg)
	}
}

func (l *Logger) printConsole(lvl Level, msg string) {
	if l.noColor {
		fmt.Printf("[%s] %s\n", lvl.String(), msg)
		return
	}

	switch lvl {
	case LevelDebug:
		fmt.Printf("%s[DEBUG]%s %s\n", colorGray, colorReset, msg)
	case LevelInfo:
		fmt.Printf("%s[INFO]%s  %s\n", colorBlue, colorReset, msg)
	case LevelWarn:
		fmt.Printf("%s[WARN]%s  %s\n", colorYellow, colorReset, msg)
	case LevelError:
		fmt.Printf("%s[ERROR]%s %s\n", colorRed, colorReset, msg)
	}
}

// Close closes the active log file handle.
func Close() {
	if globalLogger != nil {
		globalLogger.mu.Lock()
		defer globalLogger.mu.Unlock()
		if globalLogger.activeFile != nil {
			_ = globalLogger.activeFile.Close()
			globalLogger.activeFile = nil
		}
	}
}

// Public logging helpers
func Debug(format string, args ...interface{}) { Get().logMessage(LevelDebug, format, args...) }
func Info(format string, args ...interface{})  { Get().logMessage(LevelInfo, format, args...) }
func Warn(format string, args ...interface{})  { Get().logMessage(LevelWarn, format, args...) }
func Error(format string, args ...interface{}) { Get().logMessage(LevelError, format, args...) }

// Colored CLI helpers
func PrintSuccess(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if Get().noColor {
		fmt.Printf("✓ %s\n", msg)
	} else {
		fmt.Printf("%s✓%s %s\n", colorGreen, colorReset, msg)
	}
}

func PrintWarning(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if Get().noColor {
		fmt.Printf("⚠ %s\n", msg)
	} else {
		fmt.Printf("%s⚠%s %s\n", colorYellow, colorReset, msg)
	}
}

func PrintError(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if Get().noColor {
		fmt.Printf("✗ %s\n", msg)
	} else {
		fmt.Printf("%s✗%s %s\n", colorRed, colorReset, msg)
	}
}

func PrintHeader(title string) {
	width := 40
	line := strings.Repeat("─", width)
	fmt.Printf("\n%s\n%s\n", title, line)
}
