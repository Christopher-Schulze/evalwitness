package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

const baiTransientLogprobsBody = `{"error":{"message":"InvalidParameter: The parameters ` + "`logprobs`" + ` is not supported"}}`

func TestClassifyBAITransientLogprobsError(t *testing.T) {
	tests := []struct {
		name         string
		providerName string
		status       int
		body         string
		want         ErrorClass
	}{
		{name: "exact bai response", providerName: "bai", status: http.StatusBadRequest, body: baiTransientLogprobsBody, want: ClassRetryable},
		{name: "other provider", providerName: "fireworks", status: http.StatusBadRequest, body: baiTransientLogprobsBody, want: ClassCapabilityMissing},
		{name: "generic capability failure", providerName: "bai", status: http.StatusBadRequest, body: "logprobs not supported", want: ClassCapabilityMissing},
		{name: "different status", providerName: "bai", status: http.StatusNotFound, body: baiTransientLogprobsBody, want: ClassCapabilityMissing},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := classifyProviderError(test.providerName, test.status, test.body); got != test.want {
				t.Fatalf("classifyProviderError() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestBAITransientLogprobsErrorRetries(t *testing.T) {
	var calls int32
	var reservedAttempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(baiTransientLogprobsBody))
			return
		}
		writeOK(w)
	}))
	defer server.Close()

	provider, err := newOpenAI(Config{
		Name: "bai", BaseURL: server.URL, APIKey: "test", Model: "deepseek-v4-flash",
		MaxRetries: 1, LiveIntent: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := requestForTest(t, provider, "deepseek-v4-flash", 20)
	request.BeforeAttempt = func() error {
		atomic.AddInt32(&reservedAttempts, 1)
		return nil
	}

	if _, err := provider.Score(context.Background(), request); err != nil {
		t.Fatalf("expected success after transient B.AI retry: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("HTTP calls = %d, want 2", got)
	}
	if got := atomic.LoadInt32(&reservedAttempts); got != 2 {
		t.Fatalf("reserved attempts = %d, want 2", got)
	}
}
