package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	// Create Chrome container for testing
	req := testcontainers.ContainerRequest{
		Image:        "chromedp/headless-shell:136.0.7052.2",
		ExposedPorts: []string{"9222/tcp"},
		WaitingFor: wait.ForAll(
			wait.NewHTTPStrategy("/json/version").WithPort("9222/tcp").WithStatusCodeMatcher(func(status int) bool {
				return status == 200
			}),
		),
		Cmd: []string{
			"--no-sandbox",
			"--disable-gpu",
			"--remote-debugging-address=0.0.0.0",
			"--remote-debugging-port=9222",
			"--disable-extensions",
			"--enable-automation",
			"--disable-blink-features=AutomationControlled",
			"--incognito",
		},
	}

	chromeContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		log.Panicf("Failed to start Chrome container: %v", err)
	}

	defer func() {
		if err := chromeContainer.Terminate(ctx); err != nil {
			log.Panicf("Failed to terminate Chrome container: %v", err)
		}
	}()

	host, err := chromeContainer.Host(ctx)
	if err != nil {
		log.Panicf("Failed to get Chrome host: %v", err)
	}

	port, err := chromeContainer.MappedPort(ctx, "9222/tcp")
	if err != nil {
		log.Panicf("Failed to get Chrome port: %v", err)
	}

	_ = os.Setenv("CHROME_HOST", host)
	_ = os.Setenv("CHROME_PORT", port.Port())

	m.Run()
}

func TestGooglePlayStoreHandler(t *testing.T) {
	testCases := []struct {
		name           string
		method         string
		query          string
		expectedStatus int
		expectedBody   map[string]string
	}{
		{
			name:           "Valid request",
			method:         http.MethodGet,
			query:          "?bundleId=com.ninjakiwi.monkeycity&lang=en&country=US",
			expectedStatus: http.StatusOK,
			expectedBody: map[string]string{
				"bundleId":  "com.ninjakiwi.monkeycity",
				"title":     "Bloons Monkey City",
				"url":       "https://play.google.com/store/apps/details?id=com.ninjakiwi.monkeycity&hl=en&gl=US",
				"version":   "1.13",
				"updated":   "13-08-2024",
				"developer": "ninja kiwi",
			},
		},
		{
			name:           "Missing bundleId",
			method:         http.MethodGet,
			query:          "?lang=en&country=US",
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]string{
				"error": "Please provide an app bundleId",
			},
		},
		{
			name:           "Invalid bundleId",
			method:         http.MethodGet,
			query:          "?bundleId=invalid&lang=en&country=US",
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]string{
				"error": "app not found",
			},
		},
		{
			name:           "Invalid method",
			method:         http.MethodPost,
			query:          "?bundleId=com.test.app",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody: map[string]string{
				"error": "Method not allowed",
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/playstore"+tt.query, http.NoBody)
			rr := httptest.NewRecorder()

			handler := http.HandlerFunc(handleGooglePlayStore)
			handler.ServeHTTP(rr, req)

			checkResponse(t, rr, tt.expectedStatus, tt.expectedBody)
		})
	}
}

func TestAppleAppStoreHandler(t *testing.T) {
	testCases := []struct {
		name           string
		method         string
		query          string
		expectedStatus int
		expectedBody   map[string]string
	}{
		{
			name:           "Valid request",
			method:         http.MethodGet,
			query:          "?appId=1495273697&country=US",
			expectedStatus: http.StatusOK,
			expectedBody: map[string]string{
				"appId":     "1495273697",
				"bundleId":  "com.walukustudio.AlMatsurat",
				"title":     "Al Ma'tsurat",
				"url":       "https://apps.apple.com/us/app/al-matsurat/id1495273697?uo=4",
				"version":   "1.02",
				"updated":   "03-03-2020",
				"developer": "Alfan Nasrulloh",
			},
		},
		{
			name:           "Missing appId",
			method:         http.MethodGet,
			query:          "?country=US",
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]string{
				"error": "Please provide an app appId or bundleId",
			},
		},
		{
			name:           "Invalid appId",
			method:         http.MethodGet,
			query:          "?appId=invalid&country=US",
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]string{
				"error": "app not found",
			},
		},
		{
			name:           "Invalid method",
			method:         http.MethodPost,
			query:          "?appId=com.test.app",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody: map[string]string{
				"error": "Method not allowed",
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/appstore"+tt.query, http.NoBody)
			rr := httptest.NewRecorder()

			handler := http.HandlerFunc(handleAppleAppStore)
			handler.ServeHTTP(rr, req)

			checkResponse(t, rr, tt.expectedStatus, tt.expectedBody)
		})
	}
}

func TestHuaweiAppGalleryHandler(t *testing.T) {
	testCases := []struct {
		name           string
		method         string
		query          string
		expectedStatus int
		expectedBody   map[string]string
	}{
		{
			name:           "Valid request",
			method:         http.MethodGet,
			query:          "?appId=103228579",
			expectedStatus: http.StatusOK,
			expectedBody: map[string]string{
				"appId":     "103228579",
				"bundleId":  "com.scriptrepublic.checker5g",
				"title":     "5G Checker",
				"url":       "https://appgallery.huawei.com/app/C103228579",
				"version":   "1.0",
				"updated":   "09-11-2020",
				"developer": "ScriptRepublic",
			},
		},
		{
			name:           "Missing appId",
			method:         http.MethodGet,
			query:          "",
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]string{
				"error": "Please provide an app appId",
			},
		},
		{
			name:           "Invalid appId",
			method:         http.MethodGet,
			query:          "?appId=invalid",
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]string{
				"error": "app not found",
			},
		},
		{
			name:           "Invalid method",
			method:         http.MethodPost,
			query:          "?appId=1234562123",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody: map[string]string{
				"error": "Method not allowed",
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/huawei"+tt.query, http.NoBody)
			rr := httptest.NewRecorder()

			handler := http.HandlerFunc(handleHuaweiAppGallery)
			handler.ServeHTTP(rr, req)

			checkResponse(t, rr, tt.expectedStatus, tt.expectedBody)
		})
	}
}

// Helper function to check response status and body
func checkResponse(t *testing.T, response *httptest.ResponseRecorder, expectedStatus int, expectedBody map[string]string) {
	t.Helper()

	if response.Code != expectedStatus {
		t.Errorf("Expected status %d, got %d", expectedStatus, response.Code)
	}

	if expectedBody != nil {
		var actualBody map[string]string
		if err := json.NewDecoder(response.Body).Decode(&actualBody); err != nil {
			t.Fatalf("Failed to decode response body: %v", err)
		}

		for key, expectedValue := range expectedBody {
			if actualValue, exists := actualBody[key]; !exists || actualValue != expectedValue {
				t.Errorf("Expected %s to be %s, got %s", key, expectedValue, actualValue)
			}
		}
	}
}
