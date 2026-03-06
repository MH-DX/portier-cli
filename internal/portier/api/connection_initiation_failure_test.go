package portier

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReportConnectionInitiationFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/spider/connection-initiation-failure", r.URL.Path)
		require.Equal(t, "my-api-key", r.Header.Get("Authorization"))
		require.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	err := ReportConnectionInitiationFailure(server.URL, "my-api-key", ConnectionInitiationFailureRequest{
		ConnectingDeviceGUID: "00000000-0000-0000-0000-000000000000",
		ConnectionID:         "cid-1",
		ErrorCode:            "TARGET_INITIATION_ERROR",
		ErrorMessage:         "error dialing service: dial tcp",
		RecommendedAction:    "retry",
		RemoteURL:            "tcp://localhost:2222",
	})
	require.NoError(t, err)
}

func TestReportConnectionInitiationFailureReturnsErrorOnHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	err := ReportConnectionInitiationFailure(server.URL, "my-api-key", ConnectionInitiationFailureRequest{
		ConnectingDeviceGUID: "00000000-0000-0000-0000-000000000000",
		ConnectionID:         "cid-1",
		ErrorCode:            "TARGET_INITIATION_ERROR",
		ErrorMessage:         "error",
		RecommendedAction:    "retry",
		RemoteURL:            "tcp://localhost:2222",
	})
	require.Error(t, err)
}
