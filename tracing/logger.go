package tracing

import (
	"fmt"

	"github.com/uber/jaeger-client-go"
	"go.uber.org/zap"
)

type internalLogger interface {
	Infof(fmt string, a ...interface{})
	Errorf(fmt string, a ...interface{})
}

var _ jaeger.Logger = (*logAdapter)(nil)

type logAdapter struct {
	*zap.Logger
}

func (log *logAdapter) Infof(str string, args ...interface{}) {
	log.Logger.Info(fmt.Sprintf(str, args...))
}

func (log *logAdapter) Error(msg string) {
	log.Logger.Error(msg)
}
