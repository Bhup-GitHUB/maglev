package restapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchRouteHandlerRequiresValidApiKey(t *testing.T) {
	api := createTestApi(t)

	resp, model := serveApiAndRetrieveEndpoint(t, api, "/api/where/search/route.json?input=test&key=invalid")

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusUnauthorized, model.Code)
	assert.Equal(t, "permission denied", model.Text)
}

func TestSearchRouteHandlerRequiresInputParameter(t *testing.T) {
	api := createTestApi(t)

	mux := http.NewServeMux()
	api.SetRoutes(mux)
	server := httptest.NewServer(mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/where/search/route.json?key=TEST")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// Validation errors use a different response format
	var errorResponse struct {
		FieldErrors map[string][]string `json:"fieldErrors"`
	}
	err = json.NewDecoder(resp.Body).Decode(&errorResponse)
	require.NoError(t, err)

	assert.Contains(t, errorResponse.FieldErrors, "input")
	assert.Len(t, errorResponse.FieldErrors["input"], 1)
	assert.Equal(t, "input parameter is required", errorResponse.FieldErrors["input"][0])
}

func TestSearchRouteHandlerInvalidMaxCount(t *testing.T) {
	api := createTestApi(t)

	testCases := []struct {
		name     string
		maxCount string
	}{
		{"non-numeric", "abc"},
		{"negative", "-5"},
		{"zero", "0"},
		{"float", "3.5"},
		{"too-large", "999"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mux := http.NewServeMux()
			api.SetRoutes(mux)
			server := httptest.NewServer(mux)
			defer server.Close()

			resp, err := http.Get(server.URL + "/api/where/search/route.json?input=test&maxCount=" + tc.maxCount + "&key=TEST")
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

			// Validation errors use a different response format
			var errorResponse struct {
				FieldErrors map[string][]string `json:"fieldErrors"`
			}
			err = json.NewDecoder(resp.Body).Decode(&errorResponse)
			require.NoError(t, err)

			assert.Contains(t, errorResponse.FieldErrors, "maxCount")
			assert.Len(t, errorResponse.FieldErrors["maxCount"], 1)
			if tc.name == "too-large" {
				assert.Contains(t, errorResponse.FieldErrors["maxCount"][0], "must not exceed")
			} else {
				assert.Equal(t, "maxCount must be a positive integer", errorResponse.FieldErrors["maxCount"][0])
			}
		})
	}
}

func TestSearchRouteHandlerEmptyResults(t *testing.T) {
	api := createTestApi(t)

	// Search for something that won't match any routes
	resp, model := serveApiAndRetrieveEndpoint(t, api, "/api/where/search/route.json?input=zzzznonexistent&key=TEST")

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 200, model.Code)
	assert.Equal(t, "OK", model.Text)

	data, ok := model.Data.(map[string]interface{})
	require.True(t, ok)

	// Should have empty list
	list, ok := data["list"].([]interface{})
	require.True(t, ok)
	assert.Empty(t, list)

	// Check limitExceeded is false
	limitExceeded, ok := data["limitExceeded"].(bool)
	require.True(t, ok)
	assert.False(t, limitExceeded)

	// Check references structure exists
	refs, ok := data["references"].(map[string]interface{})
	require.True(t, ok)
	// For empty results, agencies should be empty but the field should exist
	_, hasAgencies := refs["agencies"]
	assert.True(t, hasAgencies, "agencies field should exist in references")
}

func TestSearchRouteHandlerSuccessfulSearch(t *testing.T) {
	api := createTestApi(t)

	// Get actual routes from the test data to find a valid search term
	routes := api.GtfsManager.GetStaticData().Routes
	require.NotEmpty(t, routes, "Test data should have at least one route")

	// Use the first route's short name or long name for search
	var searchTerm string
	for _, route := range routes {
		if route.ShortName != "" {
			searchTerm = route.ShortName
			break
		}
		if route.LongName != "" {
			// Use first word of long name
			searchTerm = route.LongName
			break
		}
	}

	if searchTerm == "" {
		t.Skip("No routes with searchable names in test data")
	}

	resp, model := serveApiAndRetrieveEndpoint(t, api, "/api/where/search/route.json?input="+searchTerm+"&key=TEST")

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 200, model.Code)
	assert.Equal(t, "OK", model.Text)

	data, ok := model.Data.(map[string]interface{})
	require.True(t, ok)

	// Should have list (may or may not have results depending on FTS index)
	list, ok := data["list"].([]interface{})
	require.True(t, ok)
	require.NotEmpty(t, list, "search should return at least one route from test data")

	route := list[0].(map[string]interface{})
	assert.NotEmpty(t, route["id"])
	assert.NotEmpty(t, route["agencyId"])
	// type should be a number
	_, ok = route["type"].(float64)
	assert.True(t, ok, "type should be a number")

	// Check references structure
	refs, ok := data["references"].(map[string]interface{})
	require.True(t, ok)
	assert.NotNil(t, refs["agencies"])
}

func TestSearchRouteHandlerMaxCountLimitsResults(t *testing.T) {
	api := createTestApi(t)

	// Use a very generic search term that might match multiple routes
	resp, model := serveApiAndRetrieveEndpoint(t, api, "/api/where/search/route.json?input=a&maxCount=1&key=TEST")

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 200, model.Code)

	data, ok := model.Data.(map[string]interface{})
	require.True(t, ok)

	list, ok := data["list"].([]interface{})
	require.True(t, ok)
	// Should be at most 1 result
	assert.LessOrEqual(t, len(list), 1)

	limitExceeded, ok := data["limitExceeded"].(bool)
	require.True(t, ok)
	if len(list) == 1 {
		assert.True(t, limitExceeded, "limitExceeded should be true when results are truncated")
	}
}

func TestSearchRouteHandlerResponseStructure(t *testing.T) {
	api := createTestApi(t)

	resp, model := serveApiAndRetrieveEndpoint(t, api, "/api/where/search/route.json?input=test&key=TEST")

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify response follows OneBusAway format
	assert.Equal(t, 200, model.Code)
	assert.NotZero(t, model.CurrentTime)
	assert.Equal(t, "OK", model.Text)
	assert.Equal(t, 2, model.Version)

	data, ok := model.Data.(map[string]interface{})
	require.True(t, ok)

	// Check required fields in data
	_, hasLimitExceeded := data["limitExceeded"]
	assert.True(t, hasLimitExceeded, "Response should have limitExceeded field")

	_, hasList := data["list"]
	assert.True(t, hasList, "Response should have list field")

	_, hasReferences := data["references"]
	assert.True(t, hasReferences, "Response should have references field")
}

func TestSearchRouteHandlerDefaultMaxCount(t *testing.T) {
	api := createTestApi(t)

	// Without maxCount parameter, should use default of 20
	resp, model := serveApiAndRetrieveEndpoint(t, api, "/api/where/search/route.json?input=a&key=TEST")

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 200, model.Code)

	data, ok := model.Data.(map[string]interface{})
	require.True(t, ok)

	list, ok := data["list"].([]interface{})
	require.True(t, ok)
	// Should be at most 20 results (default maxCount)
	assert.LessOrEqual(t, len(list), 20)
}

func TestSearchRouteHandlerContentType(t *testing.T) {
	api := createTestApi(t)

	resp, _ := serveApiAndRetrieveEndpoint(t, api, "/api/where/search/route.json?input=test&key=TEST")

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
}
