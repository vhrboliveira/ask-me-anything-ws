package api_test

import (
	"net/http"
	"testing"
)

func TestHealthcheck(t *testing.T) {
	rr := execRequest("GET", "http://localhost:5001/healthcheck", nil)

	response := rr.Result()
	defer response.Body.Close()

	assertStatusCode(t, response, http.StatusOK)

	body := parseResponseBody(t, response)

	want := "OK"
	assertResponse(t, want, string(body))
}
