package prefixed

import (
	"bytes"
	"fmt"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/mgutz/ansi"
)

const reset = ansi.Reset

var (
	baseTimestamp time.Time
	isTerminal    bool
)

func init() {
	baseTimestamp = time.Now()
	isTerminal = logrus.IsTerminal()
}

func miniTS() int {
	return int(time.Since(baseTimestamp) / time.Second)
}

type Colors struct {
	Debug   string
	Info    string
	Warn    string
	Error   string
	Prefix  string
	Default string
}

type TextFormatter struct {
	// Set to true to bypass checking for a TTY before outputting colors.
	ForceColors bool

	// Force disabling colors.
	DisableColors bool

	// Disable timestamp logging. useful when output is redirected to logging
	// system that already adds timestamps.
	DisableTimestamp bool

	// Enable logging of just the time passed since beginning of execution.
	ShortTimestamp bool

	// Timestamp format to use for display when a full timestamp is printed.
	TimestampFormat string

	// The fields are sorted by default for a consistent output. For applications
	// that log extremely frequently and don't use the JSON formatter this may not
	// be desired.
	DisableSorting bool

	// Set custom 256-bit colors for the colored output.
	// Available colors:
	// - black
	// - red
	// - green
	// - yellow
	// - blue
	// - magenta
	// - cyan
	// - white
	// - 0...255 (256 colors)
	//
	// Available attributes:
	// b = bold foreground
	// B = blink foreground
	// u = underline foreground
	// i = inverse
	//
	// Style should be formatted as: "foregroundColor+attributes:backgroundColor+attributes"
	// For example,
	// "white+u:black" - display underlined white text on black background
	// "red+b:white" - display red with bold text on white background
	Colors *Colors
}

func (f *TextFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var keys []string = make([]string, 0, len(entry.Data))
	for k := range entry.Data {
		if k != "prefix" {
			keys = append(keys, k)
		}
	}

	if !f.DisableSorting {
		sort.Strings(keys)
	}

	b := &bytes.Buffer{}

	prefixFieldClashes(entry.Data)

	isColorTerminal := isTerminal && (runtime.GOOS != "windows")
	isColored := (f.ForceColors || isColorTerminal) && !f.DisableColors

	timestampFormat := f.TimestampFormat
	if timestampFormat == "" {
		timestampFormat = time.Stamp
	}
	if isColored {
		f.printColored(b, entry, keys, timestampFormat)
	} else {
		if !f.DisableTimestamp {
			f.appendKeyValue(b, "time", entry.Time.Format(timestampFormat))
		}
		f.appendKeyValue(b, "level", entry.Level.String())
		if entry.Message != "" {
			f.appendKeyValue(b, "msg", entry.Message)
		}
		for _, key := range keys {
			f.appendKeyValue(b, key, entry.Data[key])
		}
	}

	b.WriteByte('\n')
	return b.Bytes(), nil
}

func (f *TextFormatter) printColored(b *bytes.Buffer, entry *logrus.Entry, keys []string, timestampFormat string) {
	var levelColor string
	var levelText string
	switch entry.Level {
	case logrus.DebugLevel:
		levelColor = func() string {
			c := ansi.White
			if f.Colors.Debug != "" {
				c = ansi.ColorCode(f.Colors.Debug)
			}
			return c
		}()
	case logrus.InfoLevel:
		levelColor = func() string {
			c := ansi.Blue
			if f.Colors.Info != "" {
				c = ansi.ColorCode(f.Colors.Info)
			}
			return c
		}()
	case logrus.WarnLevel:
		levelColor = func() string {
			c := ansi.Yellow
			if f.Colors.Warn != "" {
				c = ansi.ColorCode(f.Colors.Warn)
			}
			return c
		}()
	case logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel:
		levelColor = func() string {
			c := ansi.Red
			if f.Colors.Error != "" {
				c = ansi.ColorCode(f.Colors.Error)
			}
			return c
		}()
	default:
		levelColor = func() string {
			c := ansi.White
			if f.Colors.Default != "" {
				c = ansi.ColorCode(f.Colors.Default)
			}
			return c
		}()
	}

	if entry.Level != logrus.WarnLevel {
		levelText = strings.ToUpper(entry.Level.String())
	} else {
		levelText = "WARN"
	}

	prefix := ""
	message := entry.Message
	prefixColor := ansi.LightBlack
	if f.Colors.Prefix != "" {
		prefixColor = ansi.ColorCode(f.Colors.Prefix)
	}

	if prefixValue, ok := entry.Data["prefix"]; ok {
		prefix = fmt.Sprintf("%s %s:%s", prefixColor, prefixValue, reset)
	} else {
		prefixValue, trimmedMsg := extractPrefix(entry.Message)
		if prefixValue != "" {
			prefix = fmt.Sprintf("%s %s:%s", prefixColor, prefixValue, reset)
			message = trimmedMsg
		}
	}

	if f.ShortTimestamp {
		fmt.Fprintf(b, "%s[%04d]%s %s%+5s%s%s %s", prefixColor, miniTS(), reset, levelColor, levelText, reset, prefix, message)
	} else {
		fmt.Fprintf(b, "%s[%s]%s %s%+5s%s%s %s", prefixColor, entry.Time.Format(timestampFormat), reset, levelColor, levelText, reset, prefix, message)
	}
	for _, k := range keys {
		v := entry.Data[k]
		fmt.Fprintf(b, " %s%s%s=%+v", levelColor, k, reset, v)
	}
}

func needsQuoting(text string) bool {
	for _, ch := range text {
		if !((ch >= 'a' && ch <= 'z') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') ||
			ch == '-' || ch == '.') {
			return false
		}
	}
	return true
}

func extractPrefix(msg string) (string, string) {
	prefix := ""
	regex := regexp.MustCompile("^\\[(.*?)\\]")
	if regex.MatchString(msg) {
		match := regex.FindString(msg)
		prefix, msg = match[1:len(match)-1], strings.TrimSpace(msg[len(match):])
	}
	return prefix, msg
}

func (f *TextFormatter) appendKeyValue(b *bytes.Buffer, key string, value interface{}) {
	b.WriteString(key)
	b.WriteByte('=')

	switch value := value.(type) {
	case string:
		if needsQuoting(value) {
			b.WriteString(value)
		} else {
			fmt.Fprintf(b, "%q", value)
		}
	case error:
		errmsg := value.Error()
		if needsQuoting(errmsg) {
			b.WriteString(errmsg)
		} else {
			fmt.Fprintf(b, "%q", value)
		}
	default:
		fmt.Fprint(b, value)
	}

	b.WriteByte(' ')
}

func prefixFieldClashes(data logrus.Fields) {
	_, ok := data["time"]
	if ok {
		data["fields.time"] = data["time"]
	}
	_, ok = data["msg"]
	if ok {
		data["fields.msg"] = data["msg"]
	}
	_, ok = data["level"]
	if ok {
		data["fields.level"] = data["level"]
	}
}
