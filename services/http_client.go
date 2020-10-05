package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	"edge-gw-trace-service/httputil"
	"edge-gw-trace-service/tokens"
	edge_log "edge-gw-trace-service/log"
	"github.com/opentracing/opentracing-go"
	trace_log "github.com/opentracing/opentracing-go/log"
	"go.uber.org/zap"
)

const (
	StatusInternalServerErrType = "internal_server_error"
)

type Client struct {
	Client     *http.Client
	Logger     *zap.Logger
	JWTFactory *tokens.JWTTokenFactory
}

func (client *Client) doRequest(ctx context.Context, service url.URL, method string, path url.URL, header http.Header, body interface{}, response interface{}) (publicError *httputil.PublicError) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "Client.doRequest()")

	requestID := fmt.Sprintf("%s", ctx.Value(httputil.ContextKeyRequestID))
	accountID := fmt.Sprintf("%s", ctx.Value(httputil.ContextKeyAccountID))

	logger := edge_log.WithContext(ctx, client.Logger).With(zap.String("function", "doRequest()")).With(zap.String("request_id", requestID)).With(zap.String("account_id", accountID))
	urlStr := service.ResolveReference(&path).String()

	span.SetTag("span.kind", "client")
	span.SetTag("http.method", method)
	span.SetTag("http.url", urlStr)
	logger = logger.With(zap.String("method", method), zap.String("url", urlStr), zap.Any("header", header))

	defer func() {
		if publicError != nil {
			span.LogFields(
				trace_log.String("event", "error"),
				trace_log.Error(errors.New(publicError.Message)),
			)

			if publicError.Code == http.StatusInternalServerError {
				span.SetTag("error", true)
			}
		}
	}()

	var bodyReader io.Reader = http.NoBody
	var bodyBytes []byte
	var err error

	if body != nil {
		bodyBytes, err = json.Marshal(body)

		if err != nil {
			errMsg := fmt.Sprintf("Could not encode request body: %s", err)
			logger.Error("could not encode request body", zap.Error(err))

			publicError = &httputil.PublicError{
				Object   : "error",
				Code     : http.StatusInternalServerError,
				Type     : StatusInternalServerErrType,
				Message  : errMsg,
			}

			return
		}

		header.Set("Content-Type", "application/json")
		header.Set("Content-Length", fmt.Sprintf("%d", len(bodyBytes)))
		bodyReader = bytes.NewBuffer(bodyBytes)
	}

	logger = logger.With(zap.String("request_body", string(bodyBytes)))

	req, err := http.NewRequest(method, urlStr, bodyReader)

	if err != nil {
		errMsg := fmt.Sprintf("Could not create request: %s", err)
		logger.Error("could not create request", zap.Error(err))

		publicError = &httputil.PublicError{
			Object   : "error",
			Code     : http.StatusInternalServerError,
			Type     : StatusInternalServerErrType,
			Message  : errMsg,
		}

		return
	}

	for name, value := range header {
		for _, v := range value {
			req.Header.Add(name, v)
		}
	}

	opentracing.GlobalTracer().Inject(
		span.Context(),
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(req.Header))

	resp, err := client.Client.Do(req)

	if err != nil {
		errMsg := fmt.Sprintf("Could not make request: %s", err)
		logger.Error("could not make request", zap.Error(err))

		publicError = &httputil.PublicError{
			Object   : "error",
			Code     : http.StatusInternalServerError,
			Type     : StatusInternalServerErrType,
			Message  : errMsg,
		}

		return
	}

	defer resp.Body.Close()
	span.SetTag("http.status_code", resp.StatusCode)
	logger = logger.With(zap.Int("status_code", resp.StatusCode))

	var responseTarget interface{} = response

	if !httputil.IsSuccessResponse(resp) {
		publicError = &httputil.PublicError{}
		responseTarget = publicError
	}

	respBody, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		logger.Error("unable to read response body", zap.Error(err))

		if !httputil.IsSuccessResponse(resp) {
			publicError = &httputil.PublicError{
				Object   : "error",
				Code:    resp.StatusCode,
				Message: fmt.Sprintf("Unable to read error response body: %s", err),
			}
		}

		return
	}

	logger.Debug("Read response body", zap.String("response_body", string(respBody)))

	if err := json.Unmarshal(respBody, responseTarget); err != nil {
		logger.Error("unable to parse response body", zap.Error(err))

		if !httputil.IsSuccessResponse(resp) {
			publicError = &httputil.PublicError{
				Object   : "error",
				Code     : resp.StatusCode,
				Message  : fmt.Sprintf("Unable to decode error response body: %s: %s", string(respBody), err),
			}

			return
		}

		return
	}

	logger.Debug("do request")

	return
}

func (client *Client) accountToken(ctx context.Context) (string, *httputil.PublicError) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "Client.accountToken()")

	requestID := fmt.Sprintf("%s", ctx.Value(httputil.ContextKeyRequestID))
	accountID := fmt.Sprintf("%s", ctx.Value(httputil.ContextKeyAccountID))

	logger := edge_log.WithContext(ctx, client.Logger).With(zap.String("function", "accountToken()")).With(zap.String("request_id", requestID)).With(zap.String("account_id", accountID))
	logger.Debug("Generating access token")

	bearer, err := client.JWTFactory.CreateAccountToken(accountID, map[string]interface{}{
		"request_id": ctx.Value(httputil.ContextKeyRequestID),
	})

	if err != nil {
		logger.Error("Unable to generate access token", zap.Error(err))

		span.LogFields(
			trace_log.String("event", "error"),
			trace_log.String("message", "unable to generate token"),
			trace_log.Error(err),
		)

		return "", &httputil.PublicError{
			Object    : "error",
			Code      : http.StatusInternalServerError,
			Type      : StatusInternalServerErrType,
			Message   : fmt.Sprintf("Unable to generate access token: %s", err.Error()),
			RequestID : requestID,
		}
	}

	span.LogFields(
		trace_log.String("event", "token generated"),
		trace_log.String("message", "successfully generated account token"),
	)

	return bearer, nil
}
