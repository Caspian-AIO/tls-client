package tests

import (
	"fmt"
	"io"
	"testing"

	http "github.com/bogdanfinn/fhttp"
	"github.com/bogdanfinn/fhttp/httptest"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

func TestAutoPriorityHeaderHTTP1(t *testing.T) {
	tests := []struct {
		name             string
		autoPriority     bool
		expectedPriority string
	}{
		{
			name:             "auto priority enabled removes priority header on HTTP1",
			autoPriority:     true,
			expectedPriority: "",
		},
		{
			name:             "auto priority disabled preserves priority header on HTTP1",
			autoPriority:     false,
			expectedPriority: "u=0, i",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receivedPriority := make(chan string, 1)
			receivedProtocol := make(chan string, 1)

			server := httptest.NewUnstartedServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					receivedPriority <- r.Header.Get("Priority")
					receivedProtocol <- r.Proto

					w.WriteHeader(http.StatusOK)
					_, _ = fmt.Fprintln(w, "ok")
				}),
			)

			// Explicitly disable HTTP/2.
			// This guarantees that the request is served using HTTP/1.1.
			server.EnableHTTP2 = false
			server.StartTLS()
			defer server.Close()

			options := []tls_client.HttpClientOption{
				tls_client.WithClientProfile(profiles.Chrome_131),
				tls_client.WithForceHttp1(),
				tls_client.WithInsecureSkipVerify(),
			}

			if tt.autoPriority {
				options = append(
					options,
					tls_client.WithAutoPriorityHeader(),
				)
			}

			client, err := tls_client.NewHttpClient(
				tls_client.NewNoopLogger(),
				options...,
			)
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}

			req, err := http.NewRequest(
				http.MethodGet,
				server.URL,
				nil,
			)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			// Deliberately provide Priority in BOTH tests.
			//
			// WithAutoPriorityHeader enabled:
			//     HTTP/1.1 -> this should be removed.
			//
			// WithAutoPriorityHeader disabled:
			//     header should pass through untouched.
			req.Header.Set("priority", "u=0, i")

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			_, _ = io.Copy(io.Discard, resp.Body)

			if resp.StatusCode != http.StatusOK {
				t.Fatalf(
					"expected status %d, got %d",
					http.StatusOK,
					resp.StatusCode,
				)
			}

			protocol := <-receivedProtocol

			if protocol != "HTTP/1.1" {
				t.Fatalf(
					"expected HTTP/1.1, got %q",
					protocol,
				)
			}

			priority := <-receivedPriority

			if priority != tt.expectedPriority {
				t.Fatalf(
					"expected Priority header %q, got %q",
					tt.expectedPriority,
					priority,
				)
			}
		})
	}
}
