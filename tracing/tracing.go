package tracing

import (
	"fmt"
	"io"

	"github.com/opentracing/opentracing-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	"github.com/uber/jaeger-lib/metrics/prometheus"
	"go.uber.org/zap"
)

func Start(logger *zap.Logger) (io.Closer, error) {
	cfg, err := jaegercfg.FromEnv()

	if err != nil {
		return nil, fmt.Errorf("could not parse Jaeger env vars: %s", err)
	}

	tracer, closer, err := cfg.NewTracer(
		jaegercfg.Logger(&logAdapter{logger}),
		jaegercfg.Metrics(prometheus.New()),
	)

	if err != nil {
		return nil, fmt.Errorf("could not initialize jaeger tracer: %s", err)
	}

	opentracing.SetGlobalTracer(tracer)

	return closer, nil
}
