package tracing

import (
	"fmt"
	"net/http"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

func InstrumentHandler(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var serverSpan opentracing.Span
		wireContext, _ := opentracing.GlobalTracer().Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(r.Header))

		serverSpan = opentracing.StartSpan("Gateway trace", ext.RPCServerOption(wireContext))
		serverSpan.SetTag("span.kind", "server")
		serverSpan.SetTag("http.method", r.Method)

		url := r.URL.String()

		if !r.URL.IsAbs() {
			url = fmt.Sprintf("http://%s%s", r.Host, url)
		}

		serverSpan.SetTag("http.url", url)
		defer serverSpan.Finish()
		r = r.WithContext(opentracing.ContextWithSpan(r.Context(), serverSpan))
		responseWriter := &responseCodeCaptureWriter{ResponseWriter: w}

		handler(responseWriter, r)

		serverSpan.SetTag("http.status_code", responseWriter.statusCode)

		if responseWriter.statusCode == http.StatusInternalServerError {
			serverSpan.SetTag("error", true)
		}
	}
}

type responseCodeCaptureWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseCodeCaptureWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}
