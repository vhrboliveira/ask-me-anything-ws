package api_test

import (
	"net/http"
	"testing"
)

func TestHealthcheck(t *testing.T) {
	const (
		url    = "/healthcheck"
		method = http.MethodGet
	)

	rr := execRequest(t, method, url, nil)

	response := rr.Result()
	defer response.Body.Close()

	assertStatusCode(t, response, http.StatusOK)

	body := parseResponseBody(t, response)

	want := "OK"
	assertResponse(t, want, string(body))
}
