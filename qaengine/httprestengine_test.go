/*
 *  Copyright IBM Corporation 2023
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package qaengine

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLocalOnlyMiddleware(t *testing.T) {
	handler := localOnlyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	testCases := []struct {
		host     string
		expected int
	}{
		{"localhost:8080", http.StatusOK},
		{"localhost", http.StatusOK},
		{"127.0.0.1:8080", http.StatusOK},
		{"[::1]:8080", http.StatusOK},
		{"attacker.example.com:8080", http.StatusForbidden},
		{"localhost.attacker.example.com", http.StatusForbidden},
		{"10.0.0.1:8080", http.StatusForbidden},
	}
	for _, testCase := range testCases {
		req := httptest.NewRequest(http.MethodGet, currentProblemURLPrefix, nil)
		req.Host = testCase.host
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != testCase.expected {
			t.Errorf("for the host '%s' expected the status %d but got %d", testCase.host, testCase.expected, rec.Code)
		}
	}
}

func TestPostSolutionHandlerRequiresJSON(t *testing.T) {
	h := NewHTTPRESTEngine(0).(*HTTPRESTEngine)
	req := httptest.NewRequest(http.MethodPost, currentSolutionURLPrefix, strings.NewReader(`{"id":"foo","answer":"bar"}`))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	h.postSolutionHandler(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("expected the status %d but got %d", http.StatusUnsupportedMediaType, rec.Code)
	}
}
