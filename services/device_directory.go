package services

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"edge-gw-trace-service/httputil"
	"github.com/opentracing/opentracing-go"
)

// DeviceData represents a subset of the fields in a device response
type DeviceData struct {
	AccountID string `json:"account_id"`
	ID        string `json:"id"`
}

// The DeviceDirectory interface is a subset of functionality provided by the DeviceDirectory service
type DeviceDirectory interface {
	DeviceRetrieve(parentSpan opentracing.Span, ctx context.Context, id string) (DeviceData, *httputil.PublicError)
}

type DeviceDirectoryImpl struct {
	Client
	DeviceDirectoryServiceURL *url.URL
}

func (dd *DeviceDirectoryImpl) DeviceRetrieve(parentSpan opentracing.Span, ctx context.Context, id string) (DeviceData, *httputil.PublicError) {
	var deviceData DeviceData

	span := opentracing.StartSpan(
		"DeviceDirectory.DeviceRetrieve()",
		opentracing.ChildOf(parentSpan.Context()))
	defer span.Finish()

	bearer, err := dd.accountToken(ctx)

	if err != nil {
		return DeviceData{}, err
	}

	err = dd.doRequest(ctx, *dd.DeviceDirectoryServiceURL, "GET", url.URL{
		Path: fmt.Sprintf("/v3/devices/%s", id),
	}, http.Header{
		"Authorization": []string{"Bearer " + bearer},
	}, nil, &deviceData)

	return deviceData, err
}
