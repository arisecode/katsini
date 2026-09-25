package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	testValidRequest  = "Valid request"
	testInvalidMethod = "Invalid method"
)

func TestGooglePlayStoreHandler(t *testing.T) {
	testCases := []struct {
		name           string
		method         string
		query          string
		expectedStatus int
		expectedBody   map[string]string
	}{
		{
			name:           testValidRequest,
			method:         http.MethodGet,
			query:          "?bundleId=com.ninjakiwi.monkeycity&lang=en&country=US",
			expectedStatus: http.StatusOK,
			expectedBody: map[string]string{
				keyBundleID:  "com.ninjakiwi.monkeycity",
				keyTitle:     "Bloons Monkey City",
				keyURL:       "https://play.google.com/store/apps/details?id=com.ninjakiwi.monkeycity&hl=en&gl=US",
				keyVersion:   "1.13",
				keyUpdated:   "13-08-2024",
				keyDeveloper: "ninja kiwi",
			},
		},
		{
			name:           "Missing bundleId",
			method:         http.MethodGet,
			query:          "?lang=en&country=US",
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]string{
				keyError: "Please provide an app bundleId",
			},
		},
		{
			name:           "Invalid bundleId",
			method:         http.MethodGet,
			query:          "?bundleId=invalid&lang=en&country=US",
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]string{
				keyError: ErrAppNotFound.Error(),
			},
		},
		{
			name:           testInvalidMethod,
			method:         http.MethodPost,
			query:          "?bundleId=com.test.app",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody: map[string]string{
				keyError: msgMethodNotAllowed,
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), tt.method, "/playstore"+tt.query, http.NoBody)
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
			name:           testValidRequest,
			method:         http.MethodGet,
			query:          "?appId=1495273697&country=US",
			expectedStatus: http.StatusOK,
			expectedBody: map[string]string{
				keyAppID:     "1495273697",
				keyBundleID:  "com.walukustudio.AlMatsurat",
				keyTitle:     "Al Ma'tsurat",
				keyURL:       "https://apps.apple.com/us/app/al-matsurat/id1495273697?uo=4",
				keyVersion:   "1.03",
				keyUpdated:   "24-06-2026",
				keyDeveloper: "Alfan Nasrulloh",
			},
		},
		{
			name:           "Missing appId",
			method:         http.MethodGet,
			query:          "?country=US",
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]string{
				keyError: "Please provide an app appId or bundleId",
			},
		},
		{
			name:           "Invalid appId",
			method:         http.MethodGet,
			query:          "?appId=invalid&country=US",
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]string{
				keyError: ErrAppNotFound.Error(),
			},
		},
		{
			name:           testInvalidMethod,
			method:         http.MethodPost,
			query:          "?appId=com.test.app",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody: map[string]string{
				keyError: msgMethodNotAllowed,
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), tt.method, "/appstore"+tt.query, http.NoBody)
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
			name:           testValidRequest,
			method:         http.MethodGet,
			query:          "?appId=103228579",
			expectedStatus: http.StatusOK,
			expectedBody: map[string]string{
				keyAppID:     "103228579",
				keyBundleID:  checker5gBundleID,
				keyTitle:     checker5gTitle,
				keyURL:       checker5gURL,
				keyVersion:   "1.0",
				keyUpdated:   "09-11-2020",
				keyDeveloper: checker5gDeveloper,
			},
		},
		{
			name:           "Valid bundleId request",
			method:         http.MethodGet,
			query:          "?bundleId=com.scriptrepublic.checker5g",
			expectedStatus: http.StatusOK,
			expectedBody: map[string]string{
				keyAppID:     "103228579",
				keyBundleID:  checker5gBundleID,
				keyTitle:     checker5gTitle,
				keyURL:       checker5gURL,
				keyVersion:   "1.0",
				keyUpdated:   "09-11-2020",
				keyDeveloper: checker5gDeveloper,
			},
		},
		{
			name:           "Missing appId",
			method:         http.MethodGet,
			query:          "",
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]string{
				keyError: "Please provide an app appId or bundleId",
			},
		},
		{
			name:           "Invalid bundleId",
			method:         http.MethodGet,
			query:          "?bundleId=com.does.not.exist12345",
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]string{
				keyError: ErrAppNotFound.Error(),
			},
		},
		{
			name:           "Invalid appId",
			method:         http.MethodGet,
			query:          "?appId=invalid",
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]string{
				keyError: ErrAppNotFound.Error(),
			},
		},
		{
			name:           testInvalidMethod,
			method:         http.MethodPost,
			query:          "?appId=1234562123",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody: map[string]string{
				keyError: msgMethodNotAllowed,
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), tt.method, "/huawei"+tt.query, http.NoBody)
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
