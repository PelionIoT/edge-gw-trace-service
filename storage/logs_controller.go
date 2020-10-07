package storage

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/armPelionEdge/edge-gw-trace-service/httputil"
	edge_log "github.com/armPelionEdge/edge-gw-trace-service/log"

	"go.uber.org/zap"

	"github.com/opentracing/opentracing-go"
	trace_log "github.com/opentracing/opentracing-go/log"
	elastic "github.com/olivere/elastic/v7"
)

// TraceStore specifies the functions that the interface should have
type TraceStore interface {
	AddDeviceTrace(parentSpan opentracing.Span, ctx context.Context, traces []Trace) error
	SearchDeviceTrace(parentSpan opentracing.Span, ctx context.Context, query TraceQuery, includeTotalCount bool) (TracePage, error)
}

// Trace struct specifies the attibutes of device trace log
type Trace struct {
	AccountID      string `json:"account_id"`
	DeviceID       string `json:"device_id"`
	ID             string `json:"id"`
	Timestamp      int64  `json:"timestamp"`
	Timestring     string `json:"timestring"`
	Trace          map[string]interface{} `json:"trace"`
	Type           string `json:"type"`
	CloudTimestamp int64  `json:"@timestamp"`
	CreatedAt      string `json:"created_at"`
}

// TraceResponse struct specifies the attibutes of device trace
type TraceResponse struct {
	AccountID      string `json:"account_id"`
	DeviceID       string `json:"device_id"`
	ID             string `json:"id"`
	Object         string `json:"object"`
	CreatedAt      string `json:"created_at"`
	ETag           string `json:"etag"`
	Timestamp      string `json:"timestamp"`
	Trace          map[string]interface{} `json:"trace"`
	Type           string `json:"type"`
}

// TracePageecifies the return result for paginated trace data
type TracePage struct {
	Object     string          `json:"object"`
	Limit      uint64          `json:"limit"`
	After      interface{}     `json:"after"`
	Order      string          `json:"order"`
	HasMore    bool            `json:"has_more"`
	Data       []TraceResponse `json:"data"`
	TotalCount uint64          `json:"total_count,omitempty"`
}

// TraceQuery struct specifies what attributes that a trace query should have. The query would based on these terms
type TraceQuery struct {
	ID          string        `json:"id"`
	Device      []string      `json:"device_id"`
	Account     string        `json:"account_id"`
	After       time.Time     `json:"after"`
	Before      time.Time     `json:"before"`
	Type        string        `json:"type"`
	Limit       uint64        `json:"limit"`
	Sort        bool          `json:"sort"`
	AfterCursor []interface{} `json:"cursor"`
}

// ESTraceStore implements the elastic search version of the TraceStore interface
type ESTraceStore struct {
	ElasticSearchClient *elastic.Client
	ElasticSearchAlias  string
	ElasticActiveAlias  string
	Logger              *zap.Logger
}

type contextKey string

// Errors that might be returned by the functions of TraceStore
var (
	ErrCouldNotInitES           = errors.New("Failed to initial the elastic search")
	ErrNotMatchNumberOfRequests = errors.New("Found Unmatched number of actions of bulk request")
	ErrCouldNotMakeBulkRequest  = errors.New("Failed to make the bulk request")
	ErrCouldNotQueryLogs        = errors.New("Failed to query the trace logs by the specific term")
	ErrCouldNotUnmarshalLogs    = errors.New("Failed to format the query result")
	ErrCouldNotRollOverLog      = errors.New("Failed to rollover the active trace log index")
)

const (
	CtxTimeout   = time.Second * 30
)

func unixMilliseconds(t time.Time) int64 {
	return int64(t.UnixNano() / int64(time.Millisecond))
}

func encodeJSON(obj interface{}) string {
	src, err := json.Marshal(obj)
	if err != nil {
		return ""
	}
	return string(src)
}

func Date(timestamp int64) string {
	t_sec := timestamp / 1000;
	t_nsec := (timestamp % 1000) * 1000000;
	return time.Unix(t_sec, t_nsec).UTC().Format("2006-01-02T15:04:05.000Z");
}

func buildESBoolQuery(query TraceQuery) *elastic.BoolQuery {
	esQuery := elastic.NewBoolQuery()

	// Handle the device_id query term
	if len(query.Device) > 0 {
		var devices []interface{}
		for _, device := range query.Device {
			devices = append(devices, interface{}(device))
		}
		deviceQuery := elastic.NewTermsQuery("device_id", devices...)
		esQuery.Must(deviceQuery)
	}

	// Handle the account query term
	if query.Account != "" {
		accountQuery := elastic.NewTermQuery("account_id", query.Account)
		esQuery.Must(accountQuery)
	}

	// Handle the type query term
	if query.Type != "" {
		typeQuery := elastic.NewMatchQuery("type", query.Type)
		esQuery.Must(typeQuery)
	}

	// Handle the time range query term, initialize the filter function depends on the given time
	afterTime := unixMilliseconds(query.After)
	beforeTime := unixMilliseconds(query.Before)

	if !query.Before.IsZero() || !query.After.IsZero() {
		timeRangeQuery := elastic.NewRangeQuery("timestamp")

		if !query.Before.IsZero() {
			timeRangeQuery = timeRangeQuery.Lte(beforeTime)
		}

		if !query.After.IsZero() {
			timeRangeQuery = timeRangeQuery.Gte(afterTime)
		}

		esQuery.Filter(timeRangeQuery)
	}

	// Handle the trace id query term
	if query.ID != "" {
		IDQuery := elastic.NewTermQuery("id", query.ID)
		esQuery.Must(IDQuery)
	}

	return esQuery
}

func extractKeyFromContext(ctx context.Context, logger *zap.Logger) (interface{}, interface{}) {
	var requestID interface{}
	requestID = ctx.Value(httputil.ContextKeyRequestID)
	if requestID == nil {
		logger.Warn("extractKeyFromContext(): Missing requestID in the context")

		return "", ""
	}

	var accountID interface{}
	accountID = ctx.Value(httputil.ContextKeyAccountID)
	if accountID == nil {
		logger.Warn("extractKeyFromContext(): Missing accountID in the context")

		return "", ""
	}

	return requestID, accountID
}

// NewESTraceStore function initliaze an instance of ESTraceStore, establish the connection to elastic search by provided URL and index
func NewESTraceStore(logger *zap.Logger, esURL string, esSearchAlias string, esActiveAlias string) (*ESTraceStore, error) {
	client, err := elastic.NewClient(elastic.SetSniff(false), elastic.SetURL(esURL))

	if err != nil {
		logger.Error("NewESTraceStore(): An error occured inside of NewESTraceStore()", zap.Error(err))
		return &ESTraceStore{}, ErrCouldNotInitES
	}

	return &ESTraceStore{ElasticSearchClient: client, ElasticSearchAlias: esSearchAlias, ElasticActiveAlias: esActiveAlias, Logger: logger}, nil
}

// AddDeviceTrace function adds trace logs to the elastic search server which is specified by the instance of ESTraceStore
func (esTraceStore *ESTraceStore) AddDeviceTrace(parentSpan opentracing.Span, ctx context.Context, logs []Trace) error {
	// Extract the RequestID and the AccountID
	requestID, accountID := extractKeyFromContext(ctx, esTraceStore.Logger)

	span := opentracing.StartSpan(
		"ESTraceStore.AddDeviceTrace",
		opentracing.ChildOf(parentSpan.Context()))
	defer span.Finish()

	span.SetTag("component", "storage")

	logger := edge_log.WithContext(ctx, esTraceStore.Logger).With(zap.String("request_id", requestID.(string))).With(zap.String("account_id", accountID.(string))).With(zap.String("function", "AddDeviceTrace()"))

	// Create the bulk request to send the trace logs to the elasticsearch in batches
	bulkRequest := esTraceStore.ElasticSearchClient.Bulk().Index(esTraceStore.ElasticActiveAlias)
	for _, log := range logs {
		log.Timestring = Date(log.Timestamp)
		log.CreatedAt = Date(log.CloudTimestamp)
		bulkRequest.Add(elastic.NewBulkIndexRequest().Doc(log))
	}

	logger.Debug("Content of bulk request", zap.Any("content", logs))

	if bulkRequest.NumberOfActions() != len(logs) {
		logger.Warn("Fail to create bulk request", zap.Error(ErrNotMatchNumberOfRequests))

		span.LogFields(
			trace_log.String("event", "error"),
			trace_log.String("message", "failed creating bulk request"),
			trace_log.Error(ErrNotMatchNumberOfRequests),
		)

		return ErrNotMatchNumberOfRequests
	}

	// Make sure to send the bulk requests to Elasticsearch
	response, err := bulkRequest.Do(ctx)
	if err != nil {
		logger.Warn("Fail to make bulk request", zap.Error(err))

		span.LogFields(
			trace_log.String("event", "error"),
			trace_log.String("message", "bulk request failed"),
			trace_log.Error(err),
		)

		return ErrCouldNotMakeBulkRequest
	} else if response.Errors == true {
		for _, item := range response.Failed() {
			logger.Error("Bulk request failed item", zap.Any("reason", item.Error.Reason))

			span.LogFields(
				trace_log.String("event", "error"),
				trace_log.String("message", "bulk request failed with errors"),
				trace_log.Error(ErrCouldNotMakeBulkRequest),
			)

		}
		return ErrCouldNotMakeBulkRequest
	}

	logger.Debug("Response from bulk request", zap.Any("response", response))

	span.LogFields(
		trace_log.String("event", "bulk request finished"),
		trace_log.String("message", "bulk request successful"),
		trace_log.Object("response", response),
	)

	return nil
}

// SearchDeviceTrace return an object with trace logs and its base entity information, list wrapper and pagination data
func (esTraceStore *ESTraceStore) SearchDeviceTrace(parentSpan opentracing.Span, ctx context.Context, query TraceQuery, includeTotalCount bool) (TracePage, error) {
	// Extract the RequestID and the AccountID
	requestID, accountID := extractKeyFromContext(ctx, esTraceStore.Logger)

	span := opentracing.StartSpan(
		"ESTraceStore.SearchDeviceTrace",
		opentracing.ChildOf(parentSpan.Context()))
	span.SetTag("component", "storage")
	defer span.Finish()

	logger := edge_log.WithContext(ctx, esTraceStore.Logger).With(zap.String("request_id", requestID.(string))).With(zap.String("account_id", accountID.(string))).With(zap.String("function", "SearchDeviceTrace()"))

	esQuery := buildESBoolQuery(query)

	span.LogFields(
		trace_log.String("event", "build es query"),
		trace_log.String("message", "search query prepared"),
		trace_log.Object("esQuery", esQuery),
	)

	search := esTraceStore.ElasticSearchClient.Search().
		Index(esTraceStore.ElasticSearchAlias).
		Query(esQuery).
		Sort("id", query.Sort).
		From(0).
		Size(int(query.Limit) + 1) // ask for one more result than necessary to populate has_more

	if includeTotalCount {
		search.TrackTotalHits(true)
	}

	if query.AfterCursor != nil {
		search.SearchAfter(query.AfterCursor...)
	}

	result, err := search.Do(context.Background())
	if err != nil {
		logger.Warn("Error executing search query", zap.Error(err))

		span.LogFields(
			trace_log.String("event", "error"),
			trace_log.String("message", "search query failed"),
			trace_log.Error(err),
		)

		return TracePage{}, ErrCouldNotQueryLogs
	}

	var tracePage TracePage
	tracePage.Object = "list"
	tracePage.Limit = query.Limit
	if query.Sort == true {
		tracePage.Order = "ASC"
	} else {
		tracePage.Order = "DESC"
	}

	if includeTotalCount {
		tracePage.TotalCount = uint64(result.Hits.TotalHits.Value)
	}

	tracePage.HasMore = len(result.Hits.Hits) > int(tracePage.Limit)
	tracePage.Data = make([]TraceResponse, 0, len(result.Hits.Hits))

	if result.Hits.TotalHits.Value > 0 {
		for i, hit := range result.Hits.Hits {
			if i >= int(tracePage.Limit) {
				break
			}

			var trace Trace
			var traceResponse TraceResponse

			err = json.Unmarshal(hit.Source, &trace)
			if err == nil {

				traceResponse = TraceResponse {
					AccountID  : trace.AccountID,
					DeviceID   : trace.DeviceID,
					ID         : trace.ID,
					Object     : "device-trace",
					CreatedAt  : trace.CreatedAt,
					ETag       : trace.CreatedAt,
					Timestamp  : trace.Timestring,
					Trace      : trace.Trace,
					Type       : trace.Type,
				}

				tracePage.Data = append(tracePage.Data, traceResponse)
			} else {
				logger.Warn("Error decoding response as trace data: %v", zap.Error(err))

				span.LogFields(
					trace_log.String("event", "error"),
					trace_log.String("message", "could not decode es response"),
					trace_log.Error(err),
				)

				return TracePage{}, ErrCouldNotUnmarshalLogs
			}
		}
	}

	if query.AfterCursor != nil {
		tracePage.After = query.AfterCursor[0]
	}

	return tracePage, nil
}
