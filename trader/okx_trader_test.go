package trader

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newOKXTestTrader(roundTripper http.RoundTripper) *OKXTrader {
	return &OKXTrader{
		httpClient: &http.Client{Transport: roundTripper},
		retryDelay: time.Nanosecond,
	}
}

func okxHTTPResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Status:     http.StatusText(statusCode),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestOKXDoRequestRetriesTransientGETStatus(t *testing.T) {
	var attempts atomic.Int32
	trader := newOKXTestTrader(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if attempts.Add(1) < 3 {
			return okxHTTPResponse(http.StatusServiceUnavailable, "Service Unavailable"), nil
		}
		return okxHTTPResponse(http.StatusOK, `{"code":"0","msg":"","data":[{"totalEq":"1"}]}`), nil
	}))

	data, err := trader.doRequest(http.MethodGet, okxAccountPath, nil)

	require.NoError(t, err)
	require.JSONEq(t, `[{"totalEq":"1"}]`, string(data))
	require.Equal(t, int32(3), attempts.Load())
}

func TestOKXDoRequestRetriesGETTransportError(t *testing.T) {
	var attempts atomic.Int32
	trader := newOKXTestTrader(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if attempts.Add(1) == 1 {
			return nil, errors.New("Service Unavailable")
		}
		return okxHTTPResponse(http.StatusOK, `{"code":"0","msg":"","data":[]}`), nil
	}))

	_, err := trader.doRequest(http.MethodGet, okxAccountPath, nil)

	require.NoError(t, err)
	require.Equal(t, int32(2), attempts.Load())
}

func TestOKXDoRequestStopsAfterMaxGETAttempts(t *testing.T) {
	var attempts atomic.Int32
	trader := newOKXTestTrader(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts.Add(1)
		return okxHTTPResponse(http.StatusServiceUnavailable, "Service Unavailable"), nil
	}))

	_, err := trader.doRequest(http.MethodGet, okxAccountPath, nil)

	require.ErrorContains(t, err, "HTTP 503")
	require.Equal(t, int32(okxMaxGETAttempts), attempts.Load())
}

func TestOKXDoRequestDoesNotRetryPOST(t *testing.T) {
	var attempts atomic.Int32
	trader := newOKXTestTrader(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts.Add(1)
		return okxHTTPResponse(http.StatusServiceUnavailable, "Service Unavailable"), nil
	}))

	_, err := trader.doRequest(http.MethodPost, okxOrderPath, map[string]string{"instId": "BTC-USDT-SWAP"})

	require.ErrorContains(t, err, "HTTP 503")
	require.Equal(t, int32(1), attempts.Load())
}

func TestOKXDoRequestDoesNotRetryPermanentGETStatus(t *testing.T) {
	var attempts atomic.Int32
	trader := newOKXTestTrader(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts.Add(1)
		return okxHTTPResponse(http.StatusUnauthorized, `{"msg":"invalid key"}`), nil
	}))

	_, err := trader.doRequest(http.MethodGet, okxAccountPath, nil)

	require.ErrorContains(t, err, "HTTP 401")
	require.ErrorContains(t, err, "invalid key")
	require.Equal(t, int32(1), attempts.Load())
}
