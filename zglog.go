// Package log 工具包
// @Description: 工具类
package zglog

import (
	"context"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var Logger zerolog.Logger
var UartLevel zerolog.Level = 10
var AllLevelLogger FLogger

func init() {
	timeFormat := "2006-01-02 15:04:05.000"
	zerolog.TimeFieldFormat = timeFormat
	// 创建log目录
	logPath := "./logs/"
	logAbsDir, _ := filepath.Abs(logPath)
	err := os.MkdirAll(logAbsDir, os.ModePerm)
	if err != nil {
		fmt.Println("Mkdir failed, err:", err)
		return
	}
	filePrefix := filepath.Join(logPath, GetPackageName())
	AllLevelLogger = FLogger{
		Prefix:    filePrefix,
		MaxSize:   1, // MB
		LocalTime: true,
	}
	// uartlog
	//uartLogDir, _ := filepath.Abs("./logs/uart/")
	//err = os.MkdirAll(uartLogDir, os.ModePerm)
	//if err != nil {
	//	fmt.Println("Mkdir failed, err:", err)
	//	return
	//}
	//uartFilePrefix := filepath.Join(uartLogDir, "uart")
	//uartLevelLogger := &FLogger{
	//	Prefix:     uartFilePrefix,
	//	MaxSize:    100, // MB
	//	MaxAge:     1,
	//	MaxBackups: 1,
	//	LocalTime:  true,
	//}
	consoleWriter := zerolog.NewConsoleWriter()
	consoleWriter.TimeFormat = timeFormat
	// 自定义日志级别输出
	//filteredWriterWarn := &FilteredWriter{zerolog.MultiLevelWriter(uartLevelLogger), UartLevel}
	Logger = zerolog.New(zerolog.MultiLevelWriter(consoleWriter, &AllLevelLogger)).With().Caller().Stack().Timestamp().Logger()
	log.Logger = Logger
	// test
	log.Debug().Msgf("当前程序允许打印%s日志", "DBG")
	log.Info().Msgf("当前程序允许打印%s日志", "INF")
	log.Warn().Msgf("当前程序允许打印%s日志", "WRN")
	log.Trace().Msgf("当前程序允许打印%s日志", "TRC")
	log.Error().Msgf("当前程序允许打印%s日志", "ERR")

}

type FilteredWriter struct {
	w     zerolog.LevelWriter
	level zerolog.Level
}

func (w *FilteredWriter) Write(p []byte) (n int, err error) {
	return w.w.Write(p)
}

func (w *FilteredWriter) WriteLevel(level zerolog.Level, p []byte) (n int, err error) {
	if level == w.level {
		return w.w.WriteLevel(level, p)
	}
	return len(p), nil
}

// GetPackageName
//
//	@Description: 获取当前运行项目名称
//	@return string
func GetPackageName() string {
	pc, _, _, _ := runtime.Caller(1)
	parts := strings.Split(runtime.FuncForPC(pc).Name(), ".")
	pl := len(parts)
	pkgName := ""
	funcName := parts[pl-1]
	if parts[pl-2][0] == '(' {
		funcName = parts[pl-2] + "." + funcName
		pkgName = strings.Join(parts[0:pl-2], ".")
	} else {
		pkgName = strings.Join(parts[0:pl-1], ".")
	}
	dir, _ := filepath.Split(pkgName)
	dir = strings.Trim(dir, "/")
	return dir
}

// Output duplicates the global logger and sets w as its output.
func Output(w io.Writer) zerolog.Logger {
	return Logger.Output(w)
}

// With creates a child logger with the field added to its context.
func With() zerolog.Context {
	return Logger.With()
}

// Level creates a child logger with the minimum accepted level set to level.
func Level(level zerolog.Level) zerolog.Logger {
	return Logger.Level(level)
}

// Sample returns a logger with the s sampler.
func Sample(s zerolog.Sampler) zerolog.Logger {
	return Logger.Sample(s)
}

// Hook returns a logger with the h Hook.
func Hook(h zerolog.Hook) zerolog.Logger {
	return Logger.Hook(h)
}

// Err starts a new message with error level with err as a field if not nil or
// with info level if err is nil.
//
// You must call Msg on the returned event in order to send the event.
func Err(err error) *zerolog.Event {
	return Logger.Err(err)
}

// Trace starts a new message with trace level.
//
// You must call Msg on the returned event in order to send the event.
func Trace() *zerolog.Event {
	return Logger.Trace()
}

// Debug starts a new message with debug level.
//
// You must call Msg on the returned event in order to send the event.
func Debug() *zerolog.Event {
	return Logger.Debug()
}

// Info starts a new message with info level.
//
// You must call Msg on the returned event in order to send the event.
func Info() *zerolog.Event {
	return Logger.Info()
}

// Warn starts a new message with warn level.
//
// You must call Msg on the returned event in order to send the event.
func Warn() *zerolog.Event {
	return Logger.Warn()
}

// Error starts a new message with error level.
//
// You must call Msg on the returned event in order to send the event.
func Error() *zerolog.Event {
	return Logger.Error()
}

// Fatal starts a new message with fatal level. The os.Exit(1) function
// is called by the Msg method.
//
// You must call Msg on the returned event in order to send the event.
func Fatal() *zerolog.Event {
	return Logger.Fatal()
}

// Panic starts a new message with panic level. The message is also sent
// to the panic function.
//
// You must call Msg on the returned event in order to send the event.
func Panic() *zerolog.Event {
	return Logger.Panic()
}

// WithLevel starts a new message with level.
//
// You must call Msg on the returned event in order to send the event.
func WithLevel(level zerolog.Level) *zerolog.Event {
	return Logger.WithLevel(level)
}

// Log starts a new message with no level. Setting zerolog.GlobalLevel to
// zerolog.Disabled will still disable events produced by this method.
//
// You must call Msg on the returned event in order to send the event.
func Log() *zerolog.Event {
	return Logger.Log()
}

// Print sends a log event using debug level and no extra field.
// Arguments are handled in the manner of fmt.Print.
func Print(v ...interface{}) {
	Logger.Debug().CallerSkipFrame(1).Msg(fmt.Sprint(v...))
}

// Printf sends a log event using debug level and no extra field.
// Arguments are handled in the manner of fmt.Printf.
func Printf(format string, v ...interface{}) {
	Logger.Debug().CallerSkipFrame(1).Msgf(format, v...)
}

// Ctx returns the Logger associated with the ctx. If no logger
// is associated, a disabled logger is returned.
func Ctx(ctx context.Context) *zerolog.Logger {
	return zerolog.Ctx(ctx)
}
