package deps

import "github.com/sirupsen/logrus"

type Logger interface {
	Error(args ...any)
	Info(args ...any)
	WithField(key string, value any) *logrus.Entry
	WithFields(fields logrus.Fields) *logrus.Entry
}
