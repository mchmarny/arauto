package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mchmarny/artomator/pkg/pubsub"
)

const (
	invalidEvent = `{
		"action": "DELETE",
		"tag": "us-east1-docker.pkg.dev/my-project/my-repo/hello-world:1.1"
	}`
	validEvent = `{
		"action": "INSERT",
		"digest": "us-east1-docker.pkg.dev/my-project/my-repo/hello-world@sha256:6ec128e26cd5"
	}`
	sigEvent = `{
		"action": "INSERT",
		"digest": "us-east1-docker.pkg.dev/my-project/my-repo/hello-world@sha256:6ec128e26cd5",
		"tag": "us-west1-docker.pkg.dev/cloudy-demos/artomator/tester:sha256-59d78.sig"
	}`
	attEvent = `{
		"action": "INSERT",
		"digest": "us-east1-docker.pkg.dev/my-project/my-repo/hello-world@sha256:6ec128e26cd5",
		"tag": "us-west1-docker.pkg.dev/cloudy-demos/artomator/tester:sha256-59d78.att"
	}`
)

func runEventTest(t *testing.T, event string, expectedStatusCode int) {
	b, err := json.Marshal(pubsub.GetPubSubMessage("test", event))
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodPost, "/event", bytes.NewBuffer(b))
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRecorder()
	h := getTestHandler(t)
	handler := http.HandlerFunc(h.EventHandler)
	handler.ServeHTTP(r, req)
	if r.Code != http.StatusOK {
		t.Errorf("handler returned unexpected status (want:%d, got:%d)",
			http.StatusOK, r.Code)
	}
}

func TestEventHandlerWithInvalidEvent(t *testing.T) {
	runEventTest(t, invalidEvent, http.StatusOK)
}
func TestEventHandlerWithValidEvent(t *testing.T) {
	runEventTest(t, validEvent, http.StatusOK)
}
func TestEventHandlerWithSignatureEvent(t *testing.T) {
	runEventTest(t, sigEvent, http.StatusOK)
}
func TestEventHandlerWithAttestationEvent(t *testing.T) {
	runEventTest(t, attEvent, http.StatusOK)
}
