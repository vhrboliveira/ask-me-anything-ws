package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	types "github.com/vhrboliveira/ama-go/internal/utils"
)

func TestMessageReaction(t *testing.T) {
	type customFn func(method string, url string, body io.Reader) *httptest.ResponseRecorder

	const (
		baseURL = "/api/rooms/"
	)

	t.Run("adds simultaneously multiple reactions to a message", func(t *testing.T) {
		truncateData(t)

		room := createAndGetRoom(t)
		msgID, _ := createAndGetMessages(t, room.ID)
		newURL := baseURL + room.ID.String() + "/messages/" + msgID + "/react"

		var wg sync.WaitGroup
		const requests = 50

		wg.Add(requests)
		for i := 0; i < requests; i++ {
			go func() {
				defer wg.Done()
				rr := execAuthenticatedRequest(t, http.MethodPatch, newURL, nil)
				response := rr.Result()
				defer response.Body.Close()
				assert.Equal(t, http.StatusOK, response.StatusCode)
			}()
		}
		wg.Wait()

		reactions := getMessageReactions(t, msgID)

		assert.Equal(t, strconv.Itoa(reactions), strconv.Itoa(requests))
	})

	t.Run("does not remove a reaction from a message if there is no reaction", func(t *testing.T) {
		truncateData(t)

		room := createAndGetRoom(t)
		msgID, _ := createAndGetMessages(t, room.ID)
		newURL := baseURL + room.ID.String() + "/messages/" + msgID + "/react"
		rr := execAuthenticatedRequest(t, http.MethodDelete, newURL, nil)
		response := rr.Result()
		defer response.Body.Close()

		var result struct {
			Count int `json:"count"`
		}
		require.NoError(t, json.NewDecoder(response.Body).Decode(&result))

		want := 0
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, want, result.Count)
	})

	testCases := []struct {
		customSuccessName string
		customFailName    string
		unauthorizedName  [2]string
		unauthorizedFn    [2]customFn
		method            string
		expectedResult    string
	}{
		{
			customSuccessName: "adds a reaction to the message",
			customFailName:    "returns an error if fails to update message",
			unauthorizedName: [2]string{
				"returns unauthorized error if sessionID is not found",
				"returns unauthorized error if cookie is different from the session",
			},
			unauthorizedFn: [2]customFn{
				execRequestWithoutCookie,
				execRequestWithInvalidCookie,
			},
			method:         http.MethodPatch,
			expectedResult: "1",
		},
		{
			customSuccessName: "removes a reaction from the message",
			customFailName:    "returns an error if fails to remove reaction from message",
			unauthorizedName: [2]string{
				"returns unauthorized error if sessionID is not found",
				"returns unauthorized error if cookie is different from the session",
			},
			unauthorizedFn: [2]customFn{
				execRequestWithoutCookie,
				execRequestWithInvalidCookie,
			},
			method:         http.MethodDelete,
			expectedResult: "0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.customSuccessName, func(t *testing.T) {
			truncateData(t)

			room := createAndGetRoom(t)
			msgID, _ := createAndGetMessages(t, room.ID)

			if tc.method == http.MethodDelete {
				count := 1
				setMessageReaction(t, msgID, count)
			}

			newURL := baseURL + room.ID.String() + "/messages/" + msgID + "/react"

			rr := execAuthenticatedRequest(t, tc.method, newURL, nil)
			response := rr.Result()
			defer response.Body.Close()

			var result struct {
				Count int `json:"count"`
			}
			require.NoError(t, json.NewDecoder(response.Body).Decode(&result))
			assert.Equal(t, http.StatusOK, response.StatusCode)
			assert.Equal(t, tc.expectedResult, strconv.Itoa(result.Count))
		})

		t.Run(tc.customFailName, func(t *testing.T) {
			truncateData(t)

			room := createAndGetRoom(t)
			if tc.method == http.MethodPatch {
				setUpdateMessageReactionConstraintFailure(t, room.ID)
			}

			msgID, _ := createAndGetMessages(t, room.ID)
			if tc.method == http.MethodDelete {
				setDeleteMessageReactionConstraintFailure(t, room.ID, msgID)
			}
			newURL := baseURL + room.ID.String() + "/messages/" + msgID + "/react"

			rr := execAuthenticatedRequest(t, tc.method, newURL, nil)
			response := rr.Result()
			defer response.Body.Close()

			body := parseResponseBody(t, response)

			want := "error reacting to message\n"
			assert.Equal(t, response.StatusCode, http.StatusInternalServerError)
			assert.Equal(t, want, body)
		})

		t.Run("sends a message to the websocket subscribers when a reaction", func(t *testing.T) {
			truncateData(t)

			room := createAndGetRoom(t)

			server := httptest.NewServer(Router)
			defer server.Close()

			wsURL := "ws" + server.URL[4:] + "/subscribe/room/" + room.ID.String()
			ws, err := connectAuthenticatedWS(t, wsURL)
			require.NoError(t, err)
			defer ws.Close()

			msgID, _ := createAndGetMessages(t, room.ID)

			if tc.method == http.MethodDelete {
				count := 1
				setMessageReaction(t, msgID, count)
			}

			newURL := baseURL + room.ID.String() + "/messages/" + msgID + "/react"
			rr := execAuthenticatedRequest(t, tc.method, newURL, nil)
			response := rr.Result()
			defer response.Body.Close()

			var result struct {
				Count int `json:"count"`
			}
			require.NoError(t, json.NewDecoder(response.Body).Decode(&result))

			_, p, err := ws.ReadMessage()
			require.NoError(t, err)

			var receivedMessage types.Message
			var messageReactionAdded types.MessageReactionAdded
			require.NoError(t, json.Unmarshal(p, &receivedMessage))

			jsonBytes, err := json.Marshal(receivedMessage.Value)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(jsonBytes, &messageReactionAdded))

			assert.Equal(t, http.StatusOK, response.StatusCode)
			assert.Equal(t, tc.expectedResult, strconv.Itoa(result.Count))
			assert.Equal(t, msgID, messageReactionAdded.ID)
			assert.Equal(t, tc.expectedResult, strconv.Itoa(int(messageReactionAdded.Count)))
		})

		for i, test := range tc.unauthorizedName {
			t.Run(test, func(t *testing.T) {
				truncateData(t)

				fakeID := uuid.New().String()
				newURL := baseURL + fakeID + "/messages/" + fakeID + "/react"

				rr := tc.unauthorizedFn[i](tc.method, newURL, nil)
				response := rr.Result()
				defer response.Body.Close()

				body := parseResponseBody(t, response)

				want := "unauthorized, session not found or invalid\n"
				assert.Equal(t, response.StatusCode, http.StatusUnauthorized)
				assert.Equal(t, want, body)
			})
		}

		t.Run("returns an error if room id is not valid", func(t *testing.T) {
			truncateData(t)

			fakeID := uuid.New().String()
			newURL := baseURL + "invalid_room_id/messages/" + fakeID + "/react"

			rr := execAuthenticatedRequest(t, tc.method, newURL, nil)
			response := rr.Result()
			defer response.Body.Close()

			body := parseResponseBody(t, response)

			want := "invalid room id\n"
			assert.Equal(t, response.StatusCode, http.StatusBadRequest)
			assert.Equal(t, want, body)
		})

		t.Run("returns an error if room does not exist", func(t *testing.T) {
			truncateData(t)

			fakeID := uuid.New().String()
			newURL := baseURL + fakeID + "/messages/" + fakeID + "/react"

			rr := execAuthenticatedRequest(t, tc.method, newURL, nil)
			response := rr.Result()
			defer response.Body.Close()

			body := parseResponseBody(t, response)

			want := "room not found\n"
			assert.Equal(t, response.StatusCode, http.StatusBadRequest)
			assert.Equal(t, want, body)
		})

		t.Run("returns an error if message id is not valid", func(t *testing.T) {
			truncateData(t)

			room := createAndGetRoom(t)
			newURL := baseURL + room.ID.String() + "/messages/invalid_message_id/react"

			rr := execAuthenticatedRequest(t, tc.method, newURL, nil)
			response := rr.Result()
			defer response.Body.Close()

			body := parseResponseBody(t, response)

			want := "invalid message id\n"
			assert.Equal(t, response.StatusCode, http.StatusBadRequest)
			assert.Equal(t, want, body)
		})

		t.Run("returns an error if message does not exist", func(t *testing.T) {
			truncateData(t)

			room := createAndGetRoom(t)
			fakeID := uuid.New().String()
			newURL := baseURL + room.ID.String() + "/messages/" + fakeID + "/react"

			rr := execAuthenticatedRequest(t, tc.method, newURL, nil)
			response := rr.Result()
			defer response.Body.Close()

			body := parseResponseBody(t, response)

			want := "message not found\n"
			assert.Equal(t, response.StatusCode, http.StatusBadRequest)
			assert.Equal(t, want, body)
		})

		t.Run("returns an error if fails to get message", func(t *testing.T) {
			truncateData(t)
			setMessagesConstraintFailure(t)

			room := createAndGetRoom(t)
			fakeID := uuid.New().String()
			newURL := baseURL + room.ID.String() + "/messages/" + fakeID + "/react"

			rr := execAuthenticatedRequest(t, tc.method, newURL, nil)
			response := rr.Result()
			defer response.Body.Close()

			body := parseResponseBody(t, response)

			want := "error validating message ID\n"
			assert.Equal(t, response.StatusCode, http.StatusInternalServerError)
			assert.Equal(t, want, body)
		})
	}
}
