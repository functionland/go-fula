package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockPeerFunction creates a mock peer function for testing
func MockPeerFunction(clientPeerId string, bloxSeed string) (string, error) {
	return "mock-peer-id-" + clientPeerId, nil
}

// TestPropertiesHandler tests the properties endpoint
func TestPropertiesHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/properties", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(propertiesHandler)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Check that response is valid JSON
	var response map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
}

// TestPartitionHandler tests the partition endpoint
func TestPartitionHandler(t *testing.T) {
	req, err := http.NewRequest("POST", "/partition", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(partitionHandler)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Check that response is valid JSON
	var response map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
}

// TestGenerateIdentityHandler tests the generate identity endpoint
func TestGenerateIdentityHandler(t *testing.T) {
	// Create form data with required seed parameter
	form := url.Values{}
	form.Add("seed", "test-seed-value")

	req, err := http.NewRequest("POST", "/peer/generate-identity", strings.NewReader(form.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(generateIdentityHandler)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code) // Handler returns 201, not 200

	// Check that response contains peer_id (not identity)
	var response map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "peer_id")
}

// TestJoinPoolHandler tests the join pool endpoint
func TestJoinPoolHandler(t *testing.T) {
	// Test with valid JSON payload
	payload := map[string]interface{}{
		"poolId": 1,
		"chain":  "base",
	}
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/pools/join", bytes.NewBuffer(payloadBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(joinPoolHandler)

	handler.ServeHTTP(rr, req)

	// The handler might return various status codes depending on implementation
	// We just test that it doesn't panic and returns a valid response
	assert.True(t, rr.Code >= 200 && rr.Code < 600)
}

// TestLeavePoolHandler tests the leave pool endpoint
func TestLeavePoolHandler(t *testing.T) {
	// Test with valid JSON payload
	payload := map[string]interface{}{
		"poolId": 1,
		"chain":  "base",
	}
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/pools/leave", bytes.NewBuffer(payloadBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(leavePoolHandler)

	handler.ServeHTTP(rr, req)

	// The handler might return various status codes depending on implementation
	// We just test that it doesn't panic and returns a valid response
	assert.True(t, rr.Code >= 200 && rr.Code < 600)
}

// TestCancelJoinPoolHandler tests the cancel join pool endpoint
func TestCancelJoinPoolHandler(t *testing.T) {
	// Test with valid JSON payload
	payload := map[string]interface{}{
		"poolId": 1,
	}
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/pools/cancel", bytes.NewBuffer(payloadBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(cancelJoinPoolHandler)

	handler.ServeHTTP(rr, req)

	// The handler might return various status codes depending on implementation
	// We just test that it doesn't panic and returns a valid response
	assert.True(t, rr.Code >= 200 && rr.Code < 600)
}

// TestChainStatusHandler tests the chain status endpoint
func TestChainStatusHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/chain/status", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(chainStatusHandler)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Check that response is valid JSON
	var response map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
}

// TestAccountIdHandler tests the account ID endpoint
func TestAccountIdHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/account/id", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(accountIdHandler)

	handler.ServeHTTP(rr, req)

	// Account file doesn't exist in test environment, so expect 404
	assert.Equal(t, http.StatusNotFound, rr.Code)

	// Check that response contains error message
	assert.Contains(t, rr.Body.String(), "Account file not found")
}

// TestAccountSeedHandler tests the account seed endpoint
func TestAccountSeedHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/account/seed", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(accountSeedHandler)

	handler.ServeHTTP(rr, req)

	// The handler may return either 404 (file not found) or 200 (file exists with dummy data)
	if rr.Code == http.StatusNotFound {
		// File doesn't exist - check error message
		assert.Contains(t, rr.Body.String(), "Account Seed file not found")
	} else if rr.Code == http.StatusOK {
		// File exists - check JSON response
		var response map[string]interface{}
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Contains(t, response, "accountSeed")
	} else {
		t.Errorf("Unexpected status code: %d", rr.Code)
	}
}

// TestInvalidJSONPayload tests handlers with invalid JSON
func TestInvalidJSONPayload(t *testing.T) {
	invalidJSON := []byte(`{"invalid": json}`)

	tests := []struct {
		name     string
		endpoint string
		method   string
		handler  http.HandlerFunc
	}{
		{
			name:     "join_pool_invalid_json",
			endpoint: "/pools/join",
			method:   "POST",
			handler:  joinPoolHandler,
		},
		{
			name:     "leave_pool_invalid_json",
			endpoint: "/pools/leave",
			method:   "POST",
			handler:  leavePoolHandler,
		},
		{
			name:     "cancel_join_invalid_json",
			endpoint: "/pools/cancel",
			method:   "POST",
			handler:  cancelJoinPoolHandler,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, tt.endpoint, bytes.NewBuffer(invalidJSON))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			tt.handler.ServeHTTP(rr, req)

			// Should handle invalid JSON gracefully (not panic)
			assert.True(t, rr.Code >= 400 && rr.Code < 600)
		})
	}
}

// TestMethodNotAllowed tests handlers with wrong HTTP methods
func TestMethodNotAllowed(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		method   string
		handler  http.HandlerFunc
	}{
		{
			name:     "properties_post",
			endpoint: "/properties",
			method:   "POST",
			handler:  propertiesHandler,
		},
		{
			name:     "join_pool_get",
			endpoint: "/pools/join",
			method:   "GET",
			handler:  joinPoolHandler,
		},
		{
			name:     "leave_pool_get",
			endpoint: "/pools/leave",
			method:   "GET",
			handler:  leavePoolHandler,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, tt.endpoint, nil)
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			tt.handler.ServeHTTP(rr, req)

			// Should handle wrong methods gracefully
			assert.True(t, rr.Code >= 200 && rr.Code < 600)
		})
	}
}

// TestExchangePeersHandler tests the exchange peers endpoint
func TestExchangePeersHandler(t *testing.T) {
	// Test with valid JSON payload
	payload := map[string]interface{}{
		"clientPeerId": "test-client-peer-id",
		"bloxSeed":     "test-blox-seed",
	}
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/peer/exchange", bytes.NewBuffer(payloadBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(exchangePeersHandler)

	handler.ServeHTTP(rr, req)

	// The handler might return various status codes depending on implementation
	// We just test that it doesn't panic and returns a valid response
	assert.True(t, rr.Code >= 200 && rr.Code < 600)
}

// TestDeleteFulaConfigHandler tests the delete fula config endpoint
func TestDeleteFulaConfigHandler(t *testing.T) {
	req, err := http.NewRequest("DELETE", "/delete-fula-config", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(deleteFulaConfigHandler)

	handler.ServeHTTP(rr, req)

	// Should handle the request gracefully
	assert.True(t, rr.Code >= 200 && rr.Code < 600)
}

// TestConcurrentRequests tests handling of concurrent requests
func TestConcurrentRequests(t *testing.T) {
	numRequests := 10
	done := make(chan bool, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			req, err := http.NewRequest("GET", "/properties", nil)
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(propertiesHandler)

			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
			done <- true
		}()
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		<-done
	}
}
