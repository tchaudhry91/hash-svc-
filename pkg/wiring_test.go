package hasher

import (
	kitlog "github.com/go-kit/kit/log"
	kitprometheus "github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

var srv *httptest.Server

func init() {
	fieldKeys := []string{"method", "error"}
	requestCount := kitprometheus.NewCounter(stdprometheus.NewCounterVec(
		stdprometheus.CounterOpts{
			Name: "api_request_total",
			Help: "Total API requests, partitioned by method and error",
		},
		fieldKeys,
	))
	requestLatency := kitprometheus.NewHistogram(stdprometheus.NewHistogramVec(
		stdprometheus.HistogramOpts{
			Name: "request_processing_latency",
			Help: "Time taken per request, partitioned by method and error",
		},
		fieldKeys,
	))

	svc := NewHashService()
	svc = NewLoggingMiddleware(kitlog.NewJSONLogger(os.Stderr), svc)
	svc = NewInstrumentingMiddleware(requestCount, requestLatency, svc)
	hashEndpoint := MakeHashSHA256Endpoint(svc)
	router := MakeHashSHA256Handler(hashEndpoint)
	srv = httptest.NewServer(router)
}

func TestHashWiringStatusCodes(t *testing.T) {
	for _, testcase := range []struct {
		method string
		url    string
		body   string
		want   int
	}{
		{method: "POST", url: srv.URL + "/hash", body: `{"s":"world"}`, want: 200},
		{method: "POST", url: srv.URL + "/hash", body: `{"s":""}`, want: 200},
		{method: "POST", url: srv.URL + "/hash", body: `{"sdfs":""}`, want: 200},
		{method: "POST", url: srv.URL + "/hash", body: "invalid", want: 400},
		{method: "POST", url: srv.URL + "/hashDoesNotExist", body: "invalid", want: 404},
	} {
		req, _ := http.NewRequest(testcase.method, testcase.url, strings.NewReader(testcase.body))
		resp, _ := http.DefaultClient.Do(req)
		if want, have := testcase.want, resp.StatusCode; want != have {
			t.Errorf("%s %s %s: want %d, have %d", testcase.method, testcase.url, testcase.body, want, have)
		}
	}
}

func TestHashWiring(t *testing.T) {
	for _, testcase := range []struct {
		method, url, body, want string
	}{
		{"POST", srv.URL + "/hash", `{"s":"world"}`, `{"v":"486ea46224d1bb4fb680f34f7c9ad96a8f24ec88be73ea8e5a6c65260e9cb8a7"}`},
		{"POST", srv.URL + "/hash", `{"s":""}`, `{"v":"","err":"Empty input string"}`},
		{"POST", srv.URL + "/hash", `{"sdfs":""}`, `{"v":"","err":"Empty input string"}`},
	} {
		req, _ := http.NewRequest(testcase.method, testcase.url, strings.NewReader(testcase.body))
		resp, _ := http.DefaultClient.Do(req)
		body, _ := ioutil.ReadAll(resp.Body)
		if want, have := testcase.want, strings.TrimSpace(string(body)); want != have {
			t.Errorf("%s %s %s: want %q, have %q", testcase.method, testcase.url, testcase.body, want, have)
		}
	}
}
