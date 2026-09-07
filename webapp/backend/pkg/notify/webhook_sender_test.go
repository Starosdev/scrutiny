package notify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestWebhookDeliveryStatus(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusNoContent, http.StatusMultipleChoices, http.StatusBadRequest, http.StatusTooManyRequests, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			received := make(chan Payload, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
					t.Error("expected JSON POST")
				}
				var payload Payload
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
				}
				received <- payload
				w.WriteHeader(status)
			}))
			defer server.Close()
			n := Notify{Logger: logrus.New(), Payload: Payload{Subject: "temperature alert"}}
			err := n.SendWebhookNotification(server.URL)
			if status >= http.StatusMultipleChoices {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, n.Payload.Subject, (<-received).Subject)
		})
	}
}

func TestWebhookDeliveryTimeout(t *testing.T) {
	const testTimeout = 50 * time.Millisecond
	const safetyTimeout = 2 * time.Second
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))
	defer server.Close()
	defer close(release)
	n := Notify{Logger: logrus.New()}
	done := make(chan error, 1)
	go func() { done <- n.sendWebhookNotification(server.URL, &http.Client{Timeout: testTimeout}) }()
	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(safetyTimeout):
		t.Fatal("webhook delivery did not respect the client timeout")
	}
}
