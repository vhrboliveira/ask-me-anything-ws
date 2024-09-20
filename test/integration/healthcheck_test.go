package api_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealthcheck(t *testing.T) {
	const (
		url    = "/healthcheck"
		method = http.MethodGet
	)

	rr := execRequestWithoutCookie(method, url, nil)

	response := rr.Result()
	defer response.Body.Close()

	assert.Equal(t, response.StatusCode, http.StatusOK)

	body := parseResponseBody(t, response)

	want := "OK"
	assert.Equal(t, want, body)
}
