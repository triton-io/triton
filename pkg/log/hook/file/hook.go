package file

import (
	"fmt"
	"io"
	"os"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/triton-io/triton/pkg/setting"
)

// Hook is a logrus hook for ElasticSearch
type hook struct {
	out    io.Writer
	levels []logrus.Level
}

func getHost() string {
	host := os.Getenv("HOSTNAME")
	if host != "" {
		return host
	}

	return "localhost"
}

func getLogFileName() string {
	return fmt.Sprintf("/var/log/%s/log.", getHost()) + "%Y%m%d"
}

func NewHook(levels []logrus.Level) logrus.Hook {
	fileWriter, err := rotatelogs.New(
		getLogFileName(),
		rotatelogs.WithRotationTime(24*time.Duration(viper.GetInt("log.rotation_time"))*time.Hour),
		rotatelogs.WithMaxAge(24*time.Duration(viper.GetInt("log.max_age"))*time.Hour),
	)

	if err != nil {
		panic(err)
	}

	var w io.Writer = fileWriter
	if viper.GetString("log.stdout_enabled") == "true" {
		w = io.MultiWriter(os.Stdout, fileWriter)
	}

	return &hook{
		out:    w,
		levels: levels,
	}
}

func shouldFire(entry *logrus.Entry) bool {
	// fire it if entry has a setting.ShouldSendToFile field and the value is true.
	ctx := entry.Context
	if ctx == nil {
		return false
	}

	send := ctx.Value(setting.ShouldSendToFile)
	if send != nil {
		if s, ok := send.(bool); ok && s {
			return true
		}
	}
	return false
}

func (h *hook) Fire(entry *logrus.Entry) error {
	if !shouldFire(entry) {
		return nil
	}

	c, err := entry.Bytes()
	if err != nil {
		return err
	}
	_, err = h.out.Write(c)

	return err
}

func (h *hook) Levels() []logrus.Level {
	return h.levels
}
