package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"github.com/armPelionEdge/edge-gw-trace-service/httputil"
	edge_log "github.com/armPelionEdge/edge-gw-trace-service/log"
	"github.com/armPelionEdge/edge-gw-trace-service/metrics"
	"github.com/armPelionEdge/edge-gw-trace-service/services"
	"github.com/armPelionEdge/edge-gw-trace-service/storage"
	"github.com/armPelionEdge/edge-gw-trace-service/tracing"

	"go.uber.org/zap"

	"github.com/gorilla/mux"
	"github.com/opentracing/opentracing-go"
	trace_log "github.com/opentracing/opentracing-go/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/armPelionEdge/edge-gw-services-go/middleware"
	"github.com/armPelionEdge/edge-gw-services-go/token"
	"github.com/armPelionEdge/muuid-go"
)

const (
	MinLimit                    = uint64(2)
	DefaultLimit                = uint64(100)
	MaxLimit                    = uint64(1000)
	DefalutSort                 = false
	StatusInternalServerErrType = "internal_server_error"
	StatusBadRequestErrType     = "bad_request"
	StatusValidationErrType     = "validation_error"
	StatusNotFound              = "not_found"
	StatusUnauthorized          = "invalid_auth"
	MaxTimestamp int64          = 9223372036854
)

// TraceEndpoint specifies the interfaces for the gateway trace service
type TraceEndpoint struct {
	TraceStore            storage.TraceStore
	AccessTokenMiddleware mux.MiddlewareFunc
	UUIDGenerator         *muuid.MUUIDGenerator
	DeviceDirectory       services.DeviceDirectory
	Logger                *zap.Logger
}

// PostTrace struct specifies the attibutes acceptable in POST gateway_trace_service body
type PostTrace struct {
	Timestamp  string                 `json:"timestamp"`
	Trace      map[string]interface{} `json:"trace"`
	Type       string                 `json:"type"`
}

func encodeObjToString(err interface{}) string {
	encodedErr, err := json.Marshal(err)
	if err != nil {
		return ""
	}
	return string(encodedErr)
}

func encodePublicErrorObject(c int, typ string, msg string, fName string, fMsg string, id string) string {
	var pe httputil.PublicError
	pe.Object = "error"
	pe.Code = c
	pe.Type = typ
	pe.Message = msg
	pe.RequestID = id

	if fName != "" && fMsg != "" {
		f := httputil.PublicErrorField{
			Name:    fName,
			Message: fMsg,
		}

		pe.Fields = make([]httputil.PublicErrorField, 0)
		pe.Fields = append(pe.Fields, f)
	}

	b, _ := json.Marshal(pe)

	return string(b)
}

func buildContextWithValue(requestID string, accountID string) context.Context {
	ctx := context.WithValue(context.Background(), httputil.ContextKeyRequestID, requestID)
	ctx = context.WithValue(ctx, httputil.ContextKeyAccountID, accountID)

	return ctx
}

// Parse timestamp as milliseconds from mUUID
func timestampFromUUID(muuid muuid.MUUID) int64 {
	var timestamp int64 = 0

	for i := 0; i < 6; i++ {
		timestamp = (timestamp * 256) + int64(muuid[i])
	}

	return timestamp
}

func UnmarshalJSON(data []byte) ([]PostTrace, error) {
	var traces []PostTrace
	dec := json.NewDecoder(bytes.NewReader(data))

	// Disallow unknown fields to validate data
	dec.DisallowUnknownFields()

	if err := dec.Decode(&traces); err != nil {
		return []PostTrace{}, err
	}

	return traces, nil
}

func isValidUUID(uuid string) bool {
    r := regexp.MustCompile("^[a-fA-F0-9]{32}$")
    return r.MatchString(uuid)
}

// Attach function provides the rules of routes for the TraceEndpoint
func (traceEndpoint *TraceEndpoint) Attach(router *mux.Router) {
	// Route handlers
	router.HandleFunc("/", tracing.InstrumentHandler(func(w http.ResponseWriter, r *http.Request) {
		timer := prometheus.NewTimer(metrics.PrometheusPostRequestDurations)

		requestID := r.Header.Get("X-Request-ID")
		accountID := r.Header.Get("X-Account-ID")

		logger := edge_log.WithContext(r.Context(), traceEndpoint.Logger).With(zap.String("url", r.URL.String())).With(zap.String("request_id", requestID)).With(zap.String("account_id", accountID)).With(zap.String("sub-component", "add-device-trace-handler"))

		span := opentracing.SpanFromContext(r.Context())
		span.SetTag("http.method", "POST")
		span.SetTag("http.url", r.URL.String())
		span.SetTag("request_id", requestID)
		span.SetTag("account_id", accountID)
		defer span.Finish()

		span.LogFields(
			trace_log.String("event", "receive a request"),
			trace_log.String("message", "starting to handle request"),
		)

		dataStream, err := ioutil.ReadAll(r.Body)
		if err != nil {
			errMsg := fmt.Sprintf("Error reading request body: %s", err.Error())
			w.Header().Set("Content-Type", "application/json; charset=utf8")
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, encodePublicErrorObject(http.StatusBadRequest, StatusBadRequestErrType, errMsg, "", "", requestID))

			logger.Warn("Could not read request body.", zap.Error(err), zap.Int("response_code", http.StatusBadRequest))

			span.LogFields(
				trace_log.String("event", "error"),
				trace_log.String("message", "could not read request body"),
				trace_log.Error(err),
			)

			timer.ObserveDuration()
			metrics.PrometheusPostRequestErrorCounter.Inc()

			return
		}

		// Print out the request body to the logger
		logger.Debug("Read request body.", zap.String("body", string(dataStream)))

		// Decode the data stream into a list of trace logs
		logs, err := UnmarshalJSON(dataStream)
		if err != nil {
			errMsg := fmt.Sprintf("Error decoding request body: %s", err.Error())
			w.Header().Set("Content-Type", "application/json; charset=utf8")
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, encodePublicErrorObject(http.StatusBadRequest, StatusBadRequestErrType, errMsg, "", "", requestID))

			logger.Warn("Could not decode request body.", zap.Error(err), zap.Int("response_code", http.StatusBadRequest))

			span.LogFields(
				trace_log.String("event", "error"),
				trace_log.String("message", "could not decode request body as trace logs"),
				trace_log.Error(err),
			)

			timer.ObserveDuration()
			metrics.PrometheusPostRequestErrorCounter.Inc()

			return
		}

		// Handle the device id
		deviceID := r.Header.Get("X-WigWag-RelayID")
		if len(deviceID) == 0 {
			w.Header().Set("Content-Type", "application/json; charset=utf8")
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, encodePublicErrorObject(http.StatusBadRequest, StatusValidationErrType, "Empty RelayID", "X-WigWag-RelayID", "Header X-WigWag-RelayID should not be empty", requestID))

			logger.Warn("Empty RelayID.", zap.Int("response_code", http.StatusBadRequest))

			span.LogFields(
				trace_log.String("event", "error"),
				trace_log.String("message", "empty RelayID"),
			)

			timer.ObserveDuration()

			metrics.PrometheusPostRequestErrorCounter.Inc()

			return
		}

		Logs := make([]storage.Trace, len(logs))

		var MinTime time.Time = time.Unix(0, 0)
		var MaxTime time.Time = time.Unix(MaxTimestamp / 1000, (MaxTimestamp % 1000) * 1000000)

		// Assign the DeviceID, AccountID into logs
		for index,log := range logs {
			// Validate the timestamp range
			timestamp, err := time.Parse(time.RFC3339, log.Timestamp)
			if err != nil {
				fieldErr := fmt.Sprintf("Cannot parse 'timestamp' as RFC3339 string.")
				w.Header().Set("Content-Type", "application/json; charset=utf8")
				w.WriteHeader(http.StatusBadRequest)
				io.WriteString(w, encodePublicErrorObject(http.StatusBadRequest, StatusValidationErrType, "Invalid log timestamp, cannot parse as RFC3339 format.", "timestamp", fieldErr, requestID))

				logger.Warn("Invalid timestamp param in Trace body, cannot parse.", zap.Error(err), zap.Int("response_code", http.StatusBadRequest))

				span.LogFields(
					trace_log.String("event", "error"),
					trace_log.String("message", "invalid 'timestamp', cannot parse."),
					trace_log.Error(err),
				)

				timer.ObserveDuration()
				metrics.PrometheusPostRequestErrorCounter.Inc()

				return
			}

			if timestamp.Before(MinTime) || timestamp.After(MaxTime) {
				fieldErr := fmt.Sprintf("Unacceptable 'timestamp' value, not in range.")
				w.Header().Set("Content-Type", "application/json; charset=utf8")
				w.WriteHeader(http.StatusBadRequest)
				io.WriteString(w, encodePublicErrorObject(http.StatusBadRequest, StatusValidationErrType, "Invalid log timestamp.", "timestamp", fieldErr, requestID))

				logger.Warn("Invalid timestamp param in Trace body, not in range.", zap.Int("response_code", http.StatusBadRequest))

				span.LogFields(
					trace_log.String("event", "error"),
					trace_log.String("message", "invalid 'timestamp', not in range"),
				)

				timer.ObserveDuration()
				metrics.PrometheusPostRequestErrorCounter.Inc()

				return
			}

			muuid := traceEndpoint.UUIDGenerator.UUID()

			Logs[index] = storage.Trace {
				ID             : muuid.String(),
				DeviceID       : deviceID,
				AccountID      : accountID,
				Timestamp      : int64(timestamp.UnixNano() / int64(time.Millisecond)),
				CloudTimestamp : timestampFromUUID(muuid),
				Trace          : log.Trace,
				Type           : log.Type,
			}
		}

		if len(logs) > 0 {
			// Store the trace logs to the store layer
			ctx := buildContextWithValue(requestID, accountID)
			ctx, cancel := context.WithTimeout(ctx, storage.CtxTimeout)
			defer cancel()
			err = traceEndpoint.TraceStore.AddDeviceTrace(span, ctx, Logs)
			if err != nil {
				w.Header().Set("Content-Type", "application/json; charset=utf8")
				w.WriteHeader(http.StatusInternalServerError)
				io.WriteString(w, encodePublicErrorObject(http.StatusInternalServerError, StatusInternalServerErrType, err.Error(), "", "", requestID))

				logger.Error("Some error occurred inside of the AddDeviceTrace().", zap.Error(err), zap.Int("response_code", http.StatusInternalServerError))

				span.LogFields(
					trace_log.String("event", "error"),
					trace_log.String("message", "storage error occured"),
					trace_log.Error(err),
				)

				timer.ObserveDuration()
				metrics.PrometheusPostRequestErrorCounter.Inc()
				metrics.PrometheusPostRequestElasticSearchFailureCounter.Inc()

				return
			}
		} else {
			logger.Debug("There is nothing to commit.")
		}

		metrics.PrometheusPostTraceIndicator.Add(float64(len(logs)))
		timer.ObserveDuration()

		w.Header().Set("Content-Type", "application/json; charset=utf8")
		w.WriteHeader(http.StatusCreated)
		logger.Info("Success Request.", zap.Int("response_code", http.StatusCreated))

		span.LogFields(
			trace_log.String("event", "add device traces"),
			trace_log.String("message", "finished adding device traces"),
		)
	})).Methods("POST")

	// Create a subrouter for /v3 GET requests
	methods := []string{"GET"}
	v3GetRouter := router.Methods(methods...).Subrouter()

	// Add middlewares
	v3GetRouter.Use(traceEndpoint.AccessTokenMiddleware)
	v3GetRouter.Use(middleware.RequestLoggerMiddleware())

	var TraceHandler = func(span opentracing.Span, w http.ResponseWriter, r *http.Request, timer *prometheus.Timer, devices []string) {
		armAccessToken, _ := r.Context().Value(middleware.ArmAccessTokenContextKey).(token.ArmAccessToken)
		requestID := armAccessToken.RequestID
		accountID := armAccessToken.AccountID

		logger := edge_log.WithContext(r.Context(), traceEndpoint.Logger).With(zap.String("function", "TraceHandler")).With(zap.String("request_id", requestID)).With(zap.String("account_id", accountID))

		var err error
		var MinTime time.Time = time.Unix(0, 0)
		var MaxTime time.Time = time.Unix(MaxTimestamp / 1000, (MaxTimestamp % 1000) * 1000000)
		var afterTime time.Time
		var beforeTime time.Time
		var typ string
		var after []interface{}
		var include bool
		limit := DefaultLimit
		sort := DefalutSort

		query := r.URL.Query()

		var fieldErr error
		// Verify query fields
		for field := range query {
			switch field {

			case "timestamp__gte":
				// Handle the After Timestamp
				afterTime, err = time.Parse(time.RFC3339, query[field][0])
				if err != nil {
					fieldErr = errors.New("Invalid field value. Could not parse as RFC3339 format.")
				}

			case "timestamp__lte":
				// Handle the Before Timestamp
				beforeTime, err = time.Parse(time.RFC3339, query[field][0])
				if err != nil {
					fieldErr = errors.New("Invalid field value. Could not parse as RFC3339 format.")
				}

			case "type__eq":
				// Handle the type parameter
				if len(query[field][0]) != 0 {
					typ = query[field][0]
				} else {
					fieldErr = errors.New("Invalid field value ''")
				}

			case "limit":
				// Handle the limit parameter
				if len(query[field][0]) != 0 {
					limit, fieldErr = strconv.ParseUint(query[field][0], 10, 64)
					if fieldErr == nil && (limit > MaxLimit || limit < MinLimit) {
						fieldErr = errors.New("Invalid 'limit' provided. Acceptable value is 2-1000.")
					}
				} else {
					fieldErr = errors.New("Invalid field value ''")
				}

			case "order":
				// Handle the sort parameter
				if len(query[field][0]) != 0 {
					if strings.ToLower(query[field][0]) == "asc" {
						sort = true
					} else if strings.ToLower(query[field][0]) != "desc" {
						fieldErr = errors.New("Invalid 'order'. Acceptable values [ASC|DESC]")
					}
				} else {
					fieldErr = errors.New("Invalid field value ''")
				}

			case "after":
				// Handle the after parameter
				if len(query[field][0]) != 0 {
					if !isValidUUID(query[field][0]) {
						fieldErr = errors.New("Invalid after cursor.")
					} else {
						after = make([]interface{}, 1)
						after[0]  = strings.ToLower(query[field][0])
					}
				} else {
					fieldErr = errors.New("Invalid field value ''")
				}

			case "include":
				// Handle the include parameter
				if len(query[field][0]) != 0 {
					if query[field][0] == "total_count" {
						include = true
					}
				} else {
					fieldErr = errors.New("Invalid field value ''")
				}

			default:
				// Return error for invalid query field
				w.Header().Set("Content-Type", "application/json; charset=utf8")
				w.WriteHeader(http.StatusBadRequest)
				errMsg := fmt.Sprintf("Invalid field name '%s'", field)
				io.WriteString(w, encodePublicErrorObject(http.StatusBadRequest, StatusBadRequestErrType, errMsg, "", "", requestID))

				logger.Warn(errMsg, zap.Int("response_code", http.StatusBadRequest))

				span.LogFields(
					trace_log.String("event", "error"),
					trace_log.String("message", "invalid field name"),
					trace_log.String("field", field),
				)

				timer.ObserveDuration()
				metrics.PrometheusGetRequestErrorCounter.Inc()
				return
			}

			if fieldErr != nil {
				w.Header().Set("Content-Type", "application/json; charset=utf8")
				w.WriteHeader(http.StatusBadRequest)
				errMsg := fmt.Sprintf("Invalid query field '%s'", field)
				io.WriteString(w, encodePublicErrorObject(http.StatusBadRequest, StatusValidationErrType, errMsg, field, fieldErr.Error(), requestID))

				logger.Warn(errMsg, zap.Error(fieldErr), zap.Int("response_code", http.StatusBadRequest))

				span.LogFields(
					trace_log.String("event", "error"),
					trace_log.String("message", "invalid query field"),
					trace_log.String("field", field),
					trace_log.Error(fieldErr),
				)

				timer.ObserveDuration()
				metrics.PrometheusGetRequestErrorCounter.Inc()
				return
			}
		}

		if len(query["timestamp__gte"]) != 0 && len(query["timestamp__lte"]) != 0 {
			// Check the provided whether the provided before Time is after the after Time
			if beforeTime.Before(afterTime) {
				w.Header().Set("Content-Type", "application/json; charset=utf8")
				w.WriteHeader(http.StatusBadRequest)
				io.WriteString(w, encodePublicErrorObject(http.StatusBadRequest, StatusBadRequestErrType, "Invalid time range. timerange__gte should be after timerange__lte", "", "", requestID))

				logger.Warn("Invalid time range.", zap.Int("response_code", http.StatusBadRequest))

				span.LogFields(
					trace_log.String("event", "error"),
					trace_log.String("message", "invalid time query"),
				)

				timer.ObserveDuration()
				metrics.PrometheusGetRequestErrorCounter.Inc()
				return
			}
		}

		var results storage.TracePage

		// If gte > MaxTime or lte < MinTime, retuen empty page
		if afterTime.After(MaxTime) || (len(query["timestamp__lte"]) != 0 && beforeTime.Before(MinTime)) {
			results = storage.TracePage {
				Object: "list",
				HasMore: false,
				Limit: limit,
				Data: []storage.TraceResponse{},
			}

			if after != nil {
				results.After = after[0]
			}
			if sort {
				results.Order = "ASC"
			} else {
				results.Order = "DESC"
			}

			if include {
				results.TotalCount = 0
			}
		} else {
			// If lte > MaxTime, don't use the query
			if len(query["timestamp__lte"]) != 0 && beforeTime.After(MaxTime) {
				beforeTime = time.Time{}
			}

			// If gte < MinTime, don't use the query
			if len(query["timestamp__gte"]) != 0 && afterTime.Before(MinTime) {
				afterTime = time.Time{}
			}

			// Initialize the query by the parameters that handled before
			var query storage.TraceQuery
			query = storage.TraceQuery{
				Device     : devices,
				Account    : accountID,
				After      : afterTime,
				Before     : beforeTime,
				Type       : typ,
				Limit      : limit,
				Sort       : sort,
				AfterCursor: after,
			}

			logger.Debug("Sending query to storage SearchDeviceTrace()", zap.Any("query", query))
			span.LogFields(
				trace_log.String("event", "send query to storage"),
				trace_log.String("message", "Sending search query to storage"),
				trace_log.Object("query", query),
			)

			ctx := buildContextWithValue(requestID, accountID)
			results, err = traceEndpoint.TraceStore.SearchDeviceTrace(span, ctx, query, include)

			if err != nil {
				w.Header().Set("Content-Type", "application/json; charset=utf8")
				w.WriteHeader(http.StatusInternalServerError)
				io.WriteString(w, encodePublicErrorObject(http.StatusInternalServerError, StatusInternalServerErrType, err.Error(), "", "", requestID))

				logger.Error("An error occurred inside of SearchDeviceTrace().", zap.Error(err), zap.Int("response_code", http.StatusInternalServerError))

				span.LogFields(
					trace_log.String("event", "error"),
					trace_log.String("message", "storage error occured"),
					trace_log.Error(err),
				)

				timer.ObserveDuration()
				metrics.PrometheusGetRequestErrorCounter.Inc()
				metrics.PrometheusGetRequestElasticSearchFailureCounter.Inc()

				return
			}
		}

		// Encode the results into json format
		encodedResults, err := json.Marshal(results)

		if err != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf8")
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, encodePublicErrorObject(http.StatusInternalServerError, StatusInternalServerErrType, err.Error(), "", "", requestID))

			logger.Warn("Could not encode result as json.", zap.Error(err), zap.Int("response_code", http.StatusInternalServerError))
			span.LogFields(
				trace_log.String("event", "error"),
				trace_log.String("message", "could not encode as json"),
				trace_log.Error(err),
			)

			timer.ObserveDuration()
			metrics.PrometheusGetRequestErrorCounter.Inc()

			return
		}

		timer.ObserveDuration()

		w.Header().Set("Content-Type", "application/json; charset=utf8")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, string(encodedResults)+"\n")
		logger.Info("Success Request.", zap.Int("response_code", http.StatusOK))
		span.LogFields(
			trace_log.String("event", "success"),
			trace_log.String("message", "Successfully retreived trace data"),
		)

	}

	v3GetRouter.HandleFunc("/v3/device-trace{route:\\/?}", tracing.InstrumentHandler(func(w http.ResponseWriter, r *http.Request) {
		timer := prometheus.NewTimer(metrics.PrometheusGetRequestDurations)

		logger := edge_log.WithContext(r.Context(), traceEndpoint.Logger).With(zap.String("url", r.URL.String())).With(zap.String("sub-component", "get-device-trace-handler"))

		span := opentracing.SpanFromContext(r.Context())
		span.SetTag("http.method", "GET")
		span.SetTag("http.url", r.URL.String())
		defer span.Finish()

		span.LogFields(
			trace_log.String("event", "receive a request"),
			trace_log.String("message", "starting to handle request"),
		)

		armAccessToken, ok := r.Context().Value(middleware.ArmAccessTokenContextKey).(token.ArmAccessToken)
		if !ok {
			w.Header().Set("Content-Type", "application/json; charset=utf8")
			w.WriteHeader(http.StatusUnauthorized)
			io.WriteString(w, encodePublicErrorObject(http.StatusUnauthorized, StatusUnauthorized, "Unable to decode token", "", "", armAccessToken.RequestID))

			logger.Error("access token missing", zap.Int("response_code", http.StatusUnauthorized))

		span.LogFields(
			trace_log.String("event", "error"),
			trace_log.String("message", "access token missing"),
		)

			timer.ObserveDuration()
			metrics.PrometheusGetRequestErrorCounter.Inc()
			return
		}
		requestID := armAccessToken.RequestID
		accountID := armAccessToken.AccountID

		logger = logger.With(zap.String("request_id", requestID)).With(zap.String("account_id", accountID))
		span.SetTag("request_id", requestID)

		query := r.URL.Query()

		// Handle the device_id query
		var devices []string
		if len(query["device_id__in"]) != 0 {
			if query["device_id__in"][0] != "" {
				devices = strings.Split(query["device_id__in"][0], ",")
			} else {
				w.Header().Set("Content-Type", "application/json; charset=utf8")
				w.WriteHeader(http.StatusBadRequest)
				errMsg := "Invalid query field 'device_id__in'"
				io.WriteString(w, encodePublicErrorObject(http.StatusBadRequest, StatusValidationErrType, errMsg, "device_id__in", "Invalid field value ''", requestID))

				logger.Warn(errMsg, zap.Int("response_code", http.StatusBadRequest))

				span.LogFields(
					trace_log.String("event", "error"),
					trace_log.String("message", "invalid query field 'device_id__in'"),
				)

				timer.ObserveDuration()
				metrics.PrometheusGetRequestErrorCounter.Inc()
				return
			}
		}

		query.Del("device_id__in")
		r.URL.RawQuery = query.Encode()

		TraceHandler(span, w, r, timer, devices)
	})).Methods("GET")

	v3GetRouter.HandleFunc("/v3/devices/{device_id}/trace{route:\\/?}", tracing.InstrumentHandler(func(w http.ResponseWriter, r *http.Request) {
		timer := prometheus.NewTimer(metrics.PrometheusGetRequestDurations)

		logger := edge_log.WithContext(r.Context(), traceEndpoint.Logger).With(zap.String("url", r.URL.String())).With(zap.String("sub-component", "get-single-device-trace-handler"))

		span := opentracing.SpanFromContext(r.Context())
		span.SetTag("http.method", "GET")
		span.SetTag("http.url", r.URL.String())
		defer span.Finish()

		span.LogFields(
			trace_log.String("event", "receive a request"),
			trace_log.String("message", "starting to handle request"),
		)

		armAccessToken, ok := r.Context().Value(middleware.ArmAccessTokenContextKey).(token.ArmAccessToken)
		if !ok {
			w.Header().Set("Content-Type", "application/json; charset=utf8")
			w.WriteHeader(http.StatusUnauthorized)
			io.WriteString(w, encodePublicErrorObject(http.StatusUnauthorized, StatusUnauthorized, "Unable to decode token", "", "", armAccessToken.RequestID))

			logger.Error("access token missing", zap.Int("response_code", http.StatusUnauthorized))

		span.LogFields(
			trace_log.String("event", "error"),
			trace_log.String("message", "access token missing"),
		)

			timer.ObserveDuration()
			metrics.PrometheusGetRequestErrorCounter.Inc()
			return
		}
		requestID := armAccessToken.RequestID
		accountID := armAccessToken.AccountID

		logger = logger.With(zap.String("request_id", requestID)).With(zap.String("account_id", accountID))
		span.SetTag("request_id", requestID)

		params := mux.Vars(r)

		deviceID := params["device_id"]
		span.SetTag("device_id", deviceID)

		span.LogFields(
			trace_log.String("event", "device validation"),
			trace_log.String("message", "Starting to validate device id"),
		)

		// Validate device_id
		ctx := buildContextWithValue(requestID, accountID)
		deviceData, publicError := traceEndpoint.DeviceDirectory.DeviceRetrieve(span, ctx, deviceID)

		if publicError != nil {
			logger.Debug("DeviceDirectory.DeviceRetrieve responded with error.", zap.Any("error", publicError))

			span.LogFields(
				trace_log.String("event", "error"),
				trace_log.String("message", "device validation failed"),
				trace_log.Object("error", publicError),
			)

			if publicError.Code == http.StatusUnauthorized {
				publicError = &httputil.PublicError{
					Object    : "error",
					Code     : http.StatusInternalServerError,
					Type     : "internal_server_error",
					Message  : "Could not generate valid access token, error: " + publicError.Message,
				}
			}

			publicError.Message = fmt.Sprintf("Failed validating device_id: %s", publicError.Message)
			publicError.RequestID = requestID
			pe, _ := json.Marshal(publicError)

			w.Header().Set("Content-Type", "application/json; charset=utf8")
			w.WriteHeader(publicError.Code)
			io.WriteString(w, string(pe))

			timer.ObserveDuration()
			metrics.PrometheusGetRequestErrorCounter.Inc()

			return
		} else {
			logger.Debug("DeviceRetrieve: Success response", zap.Any("response", deviceData))

			span.LogFields(
				trace_log.String("event", "device validated"),
				trace_log.String("message", "device exists in account"),
				trace_log.Object("device-data", deviceData),
			)
		}

		// Handle the device_id path param
		var devices = []string { deviceID }

		TraceHandler(span, w, r, timer, devices)
	})).Methods("GET")

	v3GetRouter.HandleFunc("/v3/device-trace/{device_trace_id}{route:\\/?}", tracing.InstrumentHandler(func(w http.ResponseWriter, r *http.Request) {
		timer := prometheus.NewTimer(metrics.PrometheusGetRequestDurations)

		logger := edge_log.WithContext(r.Context(), traceEndpoint.Logger).With(zap.String("url", r.URL.String())).With(zap.String("sub-component", "get-specific-trace-handler"))

		span := opentracing.SpanFromContext(r.Context())
		span.SetTag("http.method", "GET")
		span.SetTag("http.url", r.URL.String())
		defer span.Finish()

		span.LogFields(
			trace_log.String("event", "receive a request"),
			trace_log.String("message", "starting to handle request"),
		)

		armAccessToken, ok := r.Context().Value(middleware.ArmAccessTokenContextKey).(token.ArmAccessToken)
		if !ok {
			w.Header().Set("Content-Type", "application/json; charset=utf8")
			w.WriteHeader(http.StatusUnauthorized)
			io.WriteString(w, encodePublicErrorObject(http.StatusUnauthorized, StatusUnauthorized, "Unable to decode token", "", "", armAccessToken.RequestID))

			logger.Error("access token missing", zap.Int("response_code", http.StatusUnauthorized))

		span.LogFields(
			trace_log.String("event", "error"),
			trace_log.String("message", "access token missing"),
		)

			timer.ObserveDuration()
			metrics.PrometheusGetRequestErrorCounter.Inc()
			return
		}
		requestID := armAccessToken.RequestID
		accountID := armAccessToken.AccountID

		logger = logger.With(zap.String("request_id", requestID)).With(zap.String("account_id", accountID))
		span.SetTag("request_id", requestID)

		query := r.URL.Query()
		if len(query) > 0 {
			w.Header().Set("Content-Type", "application/json; charset=utf8")
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, encodePublicErrorObject(http.StatusBadRequest, StatusBadRequestErrType, "Invalid field query", "", "", requestID))

			logger.Warn("Invalid query fields.", zap.Any("query", query), zap.Int("response_code", http.StatusBadRequest))

			span.LogFields(
				trace_log.String("event", "error"),
				trace_log.String("message", "invalid query field"),
			)

			timer.ObserveDuration()
			metrics.PrometheusGetRequestErrorCounter.Inc()
			metrics.PrometheusGetRequestElasticSearchFailureCounter.Inc()

			return
		}

		params := mux.Vars(r)
		id := params["device_trace_id"]
		span.SetTag("device_trace_id", id)

		var err error

		// Initialize the query by the parameters that handled before
		var traceQuery storage.TraceQuery
		traceQuery = storage.TraceQuery{
			Account : accountID,
			ID      : id,
			Limit   : MinLimit,
		}

		span.LogFields(
			trace_log.String("event", "sending storage query"),
			trace_log.String("message", "Sending search query to storage"),
			trace_log.Object("query", traceQuery),
		)

		logger.Debug("Sending query to storage SearchDeviceTrace()", zap.Any("query", traceQuery))
		ctx := buildContextWithValue(requestID, accountID)
		results, err := traceEndpoint.TraceStore.SearchDeviceTrace(span, ctx, traceQuery, true)

		if err != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf8")
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, encodePublicErrorObject(http.StatusInternalServerError, StatusInternalServerErrType, err.Error(), "", "", requestID))

			logger.Error("An error occurred inside of SearchDeviceTrace().", zap.Error(err), zap.Int("response_code", http.StatusInternalServerError))

			span.LogFields(
				trace_log.String("event", "error"),
				trace_log.String("message", "storage error occured"),
				trace_log.Error(err),
			)

			timer.ObserveDuration()
			metrics.PrometheusGetRequestErrorCounter.Inc()
			metrics.PrometheusGetRequestElasticSearchFailureCounter.Inc()

			return
		}

		if results.TotalCount == 0 {
			w.Header().Set("Content-Type", "application/json; charset=utf8")
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, encodePublicErrorObject(http.StatusNotFound, StatusNotFound, "Could not retreive device trace by this ID", "", "", requestID))

			logger.Warn("Could not find trace for id " + id, zap.Int("response_code", http.StatusNotFound))

			span.LogFields(
				trace_log.String("event", "error"),
				trace_log.String("message", "trace not found"),
			)

			timer.ObserveDuration()
			metrics.PrometheusGetRequestErrorCounter.Inc()

			return
		} else if results.TotalCount > 1 {
			logger.Error("Found multiple trace logs for id "+ id +". This should not have happened.")
		}

		// Encode the result  into json format
		encodedresults, err := json.Marshal(results.Data[0])

		if err != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf8")
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, encodePublicErrorObject(http.StatusInternalServerError, StatusInternalServerErrType, err.Error(), "", "", requestID))

			logger.Warn("Error encoding result as json.", zap.Error(err), zap.Int("response_code", http.StatusInternalServerError))

			span.LogFields(
				trace_log.String("event", "error"),
				trace_log.String("message", "could not encode result as json"),
				trace_log.Error(err),
			)

			timer.ObserveDuration()
			metrics.PrometheusGetRequestErrorCounter.Inc()

			return
		}

		timer.ObserveDuration()

		w.Header().Set("Content-Type", "application/json; charset=utf8")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, string(encodedresults)+"\n")
		logger.Info("Success Request.", zap.Int("response_code", http.StatusOK))

		span.LogFields(
			trace_log.String("event", "success"),
			trace_log.String("message", "successfully retreived trace"),
		)
	})).Methods("GET")

	router.HandleFunc("/liveness", func(w http.ResponseWriter, r *http.Request) {
		logger := edge_log.WithContext(r.Context(), traceEndpoint.Logger).With(zap.String("sub-component", "liveness-handler"))

		w.Header().Set("Content-Type", "application/json; charset=utf8")
		w.WriteHeader(http.StatusOK)
		logger.Info("Success Request", zap.Int("response_code", http.StatusOK))
	}).Methods("GET")

	router.HandleFunc("/readiness", func(w http.ResponseWriter, r *http.Request) {
		logger := edge_log.WithContext(r.Context(), traceEndpoint.Logger).With(zap.String("sub-component", "readiness-handler"))

		w.Header().Set("Content-Type", "application/json; charset=utf8")
		w.WriteHeader(http.StatusOK)
		logger.Info("Success Request.", zap.Int("response_code", http.StatusOK))
	}).Methods("GET")

	router.Handle("/metrics", promhttp.Handler())
}
