package log

import (
	"context"
	"fmt"
	"io/ioutil"
	"path"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/triton-io/triton/pkg/log/hook/file"
	"github.com/triton-io/triton/pkg/setting"
)

var logger *logrus.Logger

func InitLog() {
	logger = logrus.New()

	// please note the performance impact here.
	// see https://cloud.tencent.com/developer/article/1385947
	logger.SetReportCaller(true)
	logger.SetFormatter(&logrus.TextFormatter{
		// if your want to print the name of file and function correctly,
		// do not use log package directly, use log.With... instead.
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			s := strings.Split(f.Function, ".")
			funcName := s[len(s)-1]
			return funcName, fmt.Sprintf("%s/%s:%d", path.Base(path.Dir(f.File)), path.Base(f.File), f.Line)
		},
		TimestampFormat: "2006-01-02T15:04:05.999999Z07:00",
	})

	level, err := logrus.ParseLevel(viper.GetString("log.log_level"))
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)
	logger.SetOutput(ioutil.Discard)

	// add hooks
	ls := getEnabledLevels(level)

	logger.Hooks.Add(file.NewHook(ls))

	logger.Info("Logger is initialized.")
}

func getEnabledLevels(level logrus.Level) []logrus.Level {
	var levels []logrus.Level
	for _, l := range logrus.AllLevels {
		if l <= level {
			levels = append(levels, l)
		}
	}
	return levels
}

func getBaseLogger() *logrus.Entry {
	if logger == nil {
		panic("Logger is not initialized yet!")
	}
	return logger.WithContext(context.WithValue(context.Background(), setting.ShouldSendToFile, true))
}

func getLogger() *logrus.Entry {
	return getFileLogger()
}

func getFileLogger() *logrus.Entry {
	return getBaseLogger()
}

func WithError(err error) *logrus.Entry {
	return getLogger().WithError(err)
}

func WithField(key string, value interface{}) *logrus.Entry {
	return getLogger().WithField(key, value)
}

func WithFields(fields logrus.Fields) *logrus.Entry {
	return getLogger().WithFields(fields)
}

func Trace(args ...interface{}) {
	getLogger().Trace(args...)
}

func Traceln(args ...interface{}) {
	getLogger().Traceln(args...)
}

func Tracef(format string, args ...interface{}) {
	getLogger().Tracef(format, args...)
}

func Debug(args ...interface{}) {
	getLogger().Debug(args...)
}

func Debugf(format string, args ...interface{}) {
	getLogger().Debugf(format, args...)
}

func Info(args ...interface{}) {
	getLogger().Info(args...)
}

func Infoln(args ...interface{}) {
	getLogger().Infoln(args...)
}

func Infof(format string, args ...interface{}) {
	getLogger().Infof(format, args...)
}

func Warn(args ...interface{}) {
	getLogger().Warn(args...)
}

func Warnf(format string, args ...interface{}) {
	getLogger().Warnf(format, args...)
}

func Error(args ...interface{}) {
	getLogger().Error(args...)
}

func Errorln(args ...interface{}) {
	getLogger().Errorln(args...)
}

func Errorf(format string, args ...interface{}) {
	getLogger().Errorf(format, args...)
}

func Fatal(args ...interface{}) {
	getLogger().Fatal(args...)
}

func Fatalln(args ...interface{}) {
	getLogger().Fatalln(args...)
}

func Fatalf(format string, args ...interface{}) {
	getLogger().Fatalf(format, args...)
}

func Panic(args ...interface{}) {
	getLogger().Panic(args...)
}

func Panicln(args ...interface{}) {
	getLogger().Panicln(args...)
}

func Panicf(format string, args ...interface{}) {
	getLogger().Panicf(format, args...)
}
