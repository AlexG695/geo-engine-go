package geoengine

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSpatialValidatePoint_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		geofenceID   string
		lat          float64
		lng          float64
		handler      http.HandlerFunc
		expectedErr  bool
		checkErrType func(error) bool
		checkResult  func(*testing.T, *SpatialValidationResult)
	}{
		{
			name:       "successful point inside geofence",
			geofenceID: "geo-cdmx-centro",
			lat:        19.4326,
			lng:        -99.1332,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/spatial/validate" {
					t.Errorf("expected path /api/spatial/validate, got %s", r.URL.Path)
				}
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}
				if r.Header.Get("X-API-Key") != "test-spatial-key" {
					t.Errorf("expected X-API-Key 'test-spatial-key', got '%s'", r.Header.Get("X-API-Key"))
				}
				if r.Header.Get("X-Geo-Environment") != "live" {
					t.Errorf("expected X-Geo-Environment 'live', got '%s'", r.Header.Get("X-Geo-Environment"))
				}

				var req SpatialValidationRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatalf("failed to decode request: %v", err)
				}
				if req.GeofenceID != "geo-cdmx-centro" {
					t.Errorf("expected geofence_id 'geo-cdmx-centro', got '%s'", req.GeofenceID)
				}
				if req.Latitude != 19.4326 || req.Longitude != -99.1332 {
					t.Errorf("unexpected coordinates: (%f, %f)", req.Latitude, req.Longitude)
				}

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"success": true,
					"data": {
						"inside": true,
						"geofence_id": "geo-cdmx-centro",
						"geofence_name": "Zona Centro",
						"latitude": 19.4326,
						"longitude": -99.1332
					}
				}`))
			},
			expectedErr: false,
			checkResult: func(t *testing.T, res *SpatialValidationResult) {
				if !res.IsInside() {
					t.Errorf("expected IsInside() true, got false")
				}
				if res.IsOutside() {
					t.Errorf("expected IsOutside() false, got true")
				}
				if res.GeofenceName != "Zona Centro" {
					t.Errorf("expected name 'Zona Centro', got '%s'", res.GeofenceName)
				}
				lat, lng := res.Coordinates()
				if lat != 19.4326 || lng != -99.1332 {
					t.Errorf("expected coordinates (19.4326, -99.1332), got (%f, %f)", lat, lng)
				}
				if res.String() == "" {
					t.Errorf("expected non-empty String representation")
				}
			},
		},
		{
			name:       "point outside geofence",
			geofenceID: "geo-cdmx-centro",
			lat:        25.6866,
			lng:        -100.3161,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"success": true,
					"data": {
						"inside": false,
						"geofence_id": "geo-cdmx-centro",
						"geofence_name": "Zona Centro",
						"latitude": 25.6866,
						"longitude": -100.3161
					}
				}`))
			},
			expectedErr: false,
			checkResult: func(t *testing.T, res *SpatialValidationResult) {
				if res.IsInside() {
					t.Errorf("expected IsInside() false, got true")
				}
				if !res.IsOutside() {
					t.Errorf("expected IsOutside() true, got false")
				}
			},
		},
		{
			name:        "client-side error: empty geofence ID",
			geofenceID:  "   ",
			lat:         19.4326,
			lng:         -99.1332,
			expectedErr: true,
			checkErrType: func(err error) bool {
				return errors.Is(err, ErrGeofenceIDRequired)
			},
		},
		{
			name:        "client-side error: invalid latitude out of range",
			geofenceID:  "geo-1",
			lat:         95.5,
			lng:         -99.1332,
			expectedErr: true,
			checkErrType: func(err error) bool {
				return errors.Is(err, ErrInvalidLatitude)
			},
		},
		{
			name:        "client-side error: invalid longitude out of range",
			geofenceID:  "geo-1",
			lat:         19.4326,
			lng:         185.0,
			expectedErr: true,
			checkErrType: func(err error) bool {
				return errors.Is(err, ErrInvalidLongitude)
			},
		},
		{
			name:       "server returns 204 No Content mapped to ErrGeofenceNotFound",
			geofenceID: "geo-nonexistent",
			lat:        19.4326,
			lng:        -99.1332,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			},
			expectedErr: true,
			checkErrType: func(err error) bool {
				return errors.Is(err, ErrGeofenceNotFound)
			},
		},
		{
			name:       "server returns 402 Quota Exceeded",
			geofenceID: "geo-1",
			lat:        19.4326,
			lng:        -99.1332,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusPaymentRequired)
				_, _ = w.Write([]byte(`{
					"success": false,
					"error": {
						"code": 402,
						"message": "Límite de peticiones alcanzado",
						"status": "Payment Required",
						"details": [
							{
								"@type": "type.geoengineapis.com/geoengine.rpc.ErrorInfo",
								"reason": "QUOTA_EXCEEDED"
							}
						]
					}
				}`))
			},
			expectedErr: true,
			checkErrType: func(err error) bool {
				var spErr *SpatialAPIError
				return errors.As(err, &spErr) && spErr.IsQuotaExceeded() && errors.Is(err, ErrQuotaExceeded)
			},
		},
		{
			name:       "server returns 401 Unauthorized",
			geofenceID: "geo-1",
			lat:        19.4326,
			lng:        -99.1332,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{
					"success": false,
					"error": {
						"code": 401,
						"message": "Se requiere una clave de API válida",
						"status": "Unauthorized",
						"details": [
							{
								"reason": "MISSING_API_KEY"
							}
						]
					}
				}`))
			},
			expectedErr: true,
			checkErrType: func(err error) bool {
				var spErr *SpatialAPIError
				return errors.As(err, &spErr) && spErr.IsUnauthorized() && errors.Is(err, ErrUnauthorized)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var serverURL string
			if tc.handler != nil {
				server := httptest.NewServer(tc.handler)
				defer server.Close()
				serverURL = server.URL
			}

			client := New("test-spatial-key",
				WithAPIServiceURL(serverURL),
				WithTimeout(2*time.Second),
			)
			defer client.Close()

			res, err := client.Spatial.ValidatePoint(context.Background(), tc.geofenceID, tc.lat, tc.lng)

			if (err != nil) != tc.expectedErr {
				t.Fatalf("ValidatePoint() error = %v, expectedErr = %v", err, tc.expectedErr)
			}

			if tc.checkErrType != nil && !tc.checkErrType(err) {
				t.Fatalf("ValidatePoint() unexpected error type: %v", err)
			}

			if tc.checkResult != nil && res != nil {
				tc.checkResult(t, res)
			}
		})
	}
}

func TestSpatialValidateGeneral_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		lat          float64
		lng          float64
		handler      http.HandlerFunc
		expectedErr  bool
		checkErrType func(error) bool
		checkResult  func(*testing.T, *SpatialGeneralResult)
	}{
		{
			name: "point matches multiple geofences",
			lat:  19.4326,
			lng:  -99.1332,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/spatial/validate-general" {
					t.Errorf("expected path /api/spatial/validate-general, got %s", r.URL.Path)
				}

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"success": true,
					"data": {
						"inside": true,
						"total_matches": 2,
						"matched_geofences": [
							{"id": "geo-1", "name": "Zona Norte"},
							{"id": "geo-2", "name": "Distrito Federal"}
						],
						"latitude": 19.4326,
						"longitude": -99.1332
					}
				}`))
			},
			expectedErr: false,
			checkResult: func(t *testing.T, res *SpatialGeneralResult) {
				if !res.IsInside() {
					t.Errorf("expected IsInside() true")
				}
				if !res.HasMatches() {
					t.Errorf("expected HasMatches() true")
				}
				if res.TotalMatches != 2 {
					t.Errorf("expected TotalMatches 2, got %d", res.TotalMatches)
				}
				if first := res.FirstMatch(); first == nil || first.ID != "geo-1" {
					t.Errorf("expected FirstMatch 'geo-1', got %v", first)
				}

				ids := res.GeofenceIDs()
				if len(ids) != 2 || ids[0] != "geo-1" || ids[1] != "geo-2" {
					t.Errorf("unexpected IDs: %v", ids)
				}

				names := res.GeofenceNames()
				if len(names) != 2 || names[0] != "Zona Norte" || names[1] != "Distrito Federal" {
					t.Errorf("unexpected Names: %v", names)
				}

				if !res.ContainsGeofence("geo-1") || !res.ContainsGeofence("Distrito Federal") {
					t.Errorf("expected ContainsGeofence to match ID and Name")
				}
				if res.ContainsGeofence("geo-inexistente") {
					t.Errorf("unexpected match for geo-inexistente")
				}

				lat, lng := res.Coordinates()
				if lat != 19.4326 || lng != -99.1332 {
					t.Errorf("unexpected coordinates (%f, %f)", lat, lng)
				}

				if res.String() == "" {
					t.Errorf("expected non-empty String representation")
				}
			},
		},
		{
			name: "point matches zero geofences",
			lat:  0.0,
			lng:  0.0,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"success": true,
					"data": {
						"inside": false,
						"total_matches": 0,
						"matched_geofences": [],
						"latitude": 0.0,
						"longitude": 0.0
					}
				}`))
			},
			expectedErr: false,
			checkResult: func(t *testing.T, res *SpatialGeneralResult) {
				if res.IsInside() {
					t.Errorf("expected IsInside() false")
				}
				if res.HasMatches() {
					t.Errorf("expected HasMatches() false")
				}
				if res.FirstMatch() != nil {
					t.Errorf("expected FirstMatch() nil")
				}
				if len(res.GeofenceIDs()) != 0 || len(res.GeofenceNames()) != 0 {
					t.Errorf("expected empty slices for IDs and Names")
				}
			},
		},
		{
			name:        "client-side coordinate validation fails",
			lat:         -95.0,
			lng:         0.0,
			expectedErr: true,
			checkErrType: func(err error) bool {
				return errors.Is(err, ErrInvalidLatitude)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var serverURL string
			if tc.handler != nil {
				server := httptest.NewServer(tc.handler)
				defer server.Close()
				serverURL = server.URL
			}

			client := New("test-key",
				WithAPIServiceURL(serverURL),
				WithTimeout(2*time.Second),
			)
			defer client.Close()

			res, err := client.Spatial.ValidateGeneral(context.Background(), tc.lat, tc.lng)

			if (err != nil) != tc.expectedErr {
				t.Fatalf("ValidateGeneral() error = %v, expectedErr = %v", err, tc.expectedErr)
			}

			if tc.checkErrType != nil && !tc.checkErrType(err) {
				t.Fatalf("ValidateGeneral() unexpected error type: %v", err)
			}

			if tc.checkResult != nil && res != nil {
				tc.checkResult(t, res)
			}
		})
	}
}

func TestSpatialFluentBuilder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/spatial/validate":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"success": true,
				"data": {
					"inside": true,
					"geofence_id": "geo-builder-1",
					"geofence_name": "Builder Zone",
					"latitude": 19.4,
					"longitude": -99.1
				}
			}`))
		case "/api/spatial/validate-general":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"success": true,
				"data": {
					"inside": true,
					"total_matches": 1,
					"matched_geofences": [{"id": "geo-builder-1", "name": "Builder Zone"}],
					"latitude": 19.4,
					"longitude": -99.1
				}
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New("test-key", WithAPIServiceURL(server.URL))
	defer client.Close()

	ctx := context.Background()

	// 1. Single geofence via builder ExecuteSingle / Validate
	resSingle, err := client.Spatial.Check(19.4, -99.1).
		InGeofence("geo-builder-1").
		ExecuteSingle(ctx)
	if err != nil {
		t.Fatalf("builder ExecuteSingle failed: %v", err)
	}
	if !resSingle.IsInside() || resSingle.GeofenceName != "Builder Zone" {
		t.Errorf("unexpected builder single result: %v", resSingle)
	}

	// 2. Single geofence via builder strongly typed Execute
	resGeneric, err := client.Spatial.Check(19.4, -99.1).
		InGeofence("geo-builder-1").
		Execute(ctx)
	if err != nil {
		t.Fatalf("builder Execute failed: %v", err)
	}
	if !resGeneric.IsInside() || resGeneric.GeofenceName != "Builder Zone" {
		t.Errorf("unexpected builder single result: %v", resGeneric)
	}

	// 3. General geofences via builder ExecuteGeneral
	resGeneral, err := client.Spatial.Check(19.4, -99.1).
		InAllGeofences().
		ExecuteGeneral(ctx)
	if err != nil {
		t.Fatalf("builder ExecuteGeneral failed: %v", err)
	}
	if resGeneral.TotalMatches != 1 {
		t.Errorf("unexpected general matches: %d", resGeneral.TotalMatches)
	}

	// 4. General geofences via builder strongly typed Execute
	resGenericAll, err := client.Spatial.Check(19.4, -99.1).
		InAllGeofences().
		Execute(ctx)
	if err != nil {
		t.Fatalf("builder general Execute failed: %v", err)
	}
	if resGenericAll.TotalMatches != 1 {
		t.Errorf("unexpected general matches: %d", resGenericAll.TotalMatches)
	}
}

func TestClientSpatialDirectShortcuts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/spatial/validate":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"success": true,
				"data": {
					"inside": true,
					"geofence_id": "geo-direct",
					"latitude": 10.0,
					"longitude": 20.0
				}
			}`))
		case "/api/spatial/validate-general":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"success": true,
				"data": {
					"inside": true,
					"total_matches": 1,
					"matched_geofences": [{"id": "geo-direct", "name": "Direct"}],
					"latitude": 10.0,
					"longitude": 20.0
				}
			}`))
		}
	}))
	defer server.Close()

	client := New("key", WithAPIServiceURL(server.URL))
	defer client.Close()

	ctx := context.Background()

	// Direct shortcuts on client
	res1, err := client.ValidateSpatialPoint(ctx, "geo-direct", 10.0, 20.0)
	if err != nil || !res1.IsInside() {
		t.Fatalf("client.ValidateSpatialPoint failed: %v", err)
	}

	res2, err := client.ValidateSpatialLocation(ctx, SpatialValidationRequest{
		GeofenceID: "geo-direct",
		Latitude:   10.0,
		Longitude:  20.0,
	})
	if err != nil || !res2.IsInside() {
		t.Fatalf("client.ValidateSpatialLocation failed: %v", err)
	}

	res3, err := client.ValidateSpatialGeneral(ctx, 10.0, 20.0)
	if err != nil || !res3.IsInside() {
		t.Fatalf("client.ValidateSpatialGeneral failed: %v", err)
	}

	res4, err := client.ValidateSpatialGeneralLocation(ctx, SpatialGeneralRequest{
		Latitude:  10.0,
		Longitude: 20.0,
	})
	if err != nil || !res4.IsInside() {
		t.Fatalf("client.ValidateSpatialGeneralLocation failed: %v", err)
	}
}

func TestSpatialAPIError_Methods(t *testing.T) {
	err := &SpatialAPIError{
		StatusCode: http.StatusNotFound,
		Code:       "NOT_FOUND",
		Message:    "geofence not found in database",
	}

	if !err.IsNotFound() {
		t.Errorf("expected IsNotFound() true")
	}
	if !errors.Is(err, ErrGeofenceNotFound) {
		t.Errorf("expected errors.Is(err, ErrGeofenceNotFound) true")
	}
	if err.IsQuotaExceeded() {
		t.Errorf("expected IsQuotaExceeded() false")
	}
	if err.Error() == "" {
		t.Errorf("expected non-empty Error()")
	}

	quotaErr := &SpatialAPIError{
		StatusCode: http.StatusPaymentRequired,
		Code:       "QUOTA_EXCEEDED",
		Message:    "plan limit exceeded",
	}
	if !quotaErr.IsQuotaExceeded() || !errors.Is(quotaErr, ErrQuotaExceeded) {
		t.Errorf("expected quota error detection")
	}

	unauthErr := &SpatialAPIError{
		StatusCode: http.StatusUnauthorized,
		Code:       "UNAUTHORIZED",
		Message:    "unauthorized key",
	}
	if !unauthErr.IsUnauthorized() || !errors.Is(unauthErr, ErrUnauthorized) {
		t.Errorf("expected unauthorized detection")
	}

	forbiddenErr := &SpatialAPIError{
		StatusCode: http.StatusForbidden,
		Message:    "forbidden access",
	}
	if !errors.Is(forbiddenErr, ErrForbidden) {
		t.Errorf("expected forbidden detection")
	}
}

func TestValidateCoordinates(t *testing.T) {
	if err := ValidateCoordinates(19.4326, -99.1332); err != nil {
		t.Errorf("expected valid coordinates, got: %v", err)
	}
	if err := ValidateCoordinates(91.0, 0.0); !errors.Is(err, ErrInvalidLatitude) {
		t.Errorf("expected ErrInvalidLatitude, got: %v", err)
	}
	if err := ValidateCoordinates(-91.0, 0.0); !errors.Is(err, ErrInvalidLatitude) {
		t.Errorf("expected ErrInvalidLatitude, got: %v", err)
	}
	if err := ValidateCoordinates(0.0, 181.0); !errors.Is(err, ErrInvalidLongitude) {
		t.Errorf("expected ErrInvalidLongitude, got: %v", err)
	}
	if err := ValidateCoordinates(0.0, -181.0); !errors.Is(err, ErrInvalidLongitude) {
		t.Errorf("expected ErrInvalidLongitude, got: %v", err)
	}
}

func TestSpatialRequest_Validate(t *testing.T) {
	req := SpatialValidationRequest{
		GeofenceID: "geo-1",
		Latitude:   10.0,
		Longitude:  20.0,
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected valid request, got: %v", err)
	}

	reqEmpty := SpatialValidationRequest{
		GeofenceID: "   ",
		Latitude:   10.0,
		Longitude:  20.0,
	}
	if err := reqEmpty.Validate(); !errors.Is(err, ErrGeofenceIDRequired) {
		t.Errorf("expected ErrGeofenceIDRequired, got: %v", err)
	}

	genReq := SpatialGeneralRequest{
		Latitude:  10.0,
		Longitude: 20.0,
	}
	if err := genReq.Validate(); err != nil {
		t.Errorf("expected valid request, got: %v", err)
	}
}

func TestSpatialGeneralResult_FindMatch(t *testing.T) {
	res := &SpatialGeneralResult{
		Inside:       true,
		TotalMatches: 2,
		MatchedGeofences: []GeofenceMatch{
			{ID: "geo-cdmx", Name: "CDMX Central"},
			{ID: "geo-mty", Name: "Monterrey Norte"},
		},
	}

	match1, found1 := res.FindMatch("geo-cdmx")
	if !found1 || match1 == nil || match1.Name != "CDMX Central" {
		t.Errorf("expected to find geo-cdmx, got: %v, %v", match1, found1)
	}

	match2, found2 := res.FindMatch("Monterrey Norte")
	if !found2 || match2 == nil || match2.ID != "geo-mty" {
		t.Errorf("expected to find Monterrey Norte by name, got: %v, %v", match2, found2)
	}

	_, found3 := res.FindMatch("nonexistent")
	if found3 {
		t.Errorf("expected not found for nonexistent match")
	}

	var nilRes *SpatialGeneralResult
	_, foundNil := nilRes.FindMatch("anything")
	if foundNil {
		t.Errorf("expected false on nil result")
	}
}

func TestSpatialAPIError_Unwrap(t *testing.T) {
	err404 := &SpatialAPIError{StatusCode: http.StatusNotFound, Code: "NOT_FOUND"}
	if !errors.Is(err404, ErrGeofenceNotFound) {
		t.Errorf("expected Unwrap to match ErrGeofenceNotFound")
	}

	err402 := &SpatialAPIError{StatusCode: http.StatusPaymentRequired}
	if !errors.Is(err402, ErrQuotaExceeded) {
		t.Errorf("expected Unwrap to match ErrQuotaExceeded")
	}

	err401 := &SpatialAPIError{StatusCode: http.StatusUnauthorized}
	if !errors.Is(err401, ErrUnauthorized) {
		t.Errorf("expected Unwrap to match ErrUnauthorized")
	}

	err403 := &SpatialAPIError{StatusCode: http.StatusForbidden}
	if !errors.Is(err403, ErrForbidden) {
		t.Errorf("expected Unwrap to match ErrForbidden")
	}
}

func BenchmarkValidateCoordinates(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = ValidateCoordinates(19.4326, -99.1332)
	}
}

func BenchmarkSpatialValidationResult_String(b *testing.B) {
	res := &SpatialValidationResult{
		Inside:       true,
		GeofenceID:   "geo-123",
		GeofenceName: "Distrito Federal",
		Latitude:     19.4326,
		Longitude:    -99.1332,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = res.String()
	}
}

func BenchmarkSpatialGeneralResult_Helpers(b *testing.B) {
	res := &SpatialGeneralResult{
		Inside:       true,
		TotalMatches: 3,
		MatchedGeofences: []GeofenceMatch{
			{ID: "geo-1", Name: "Zone A"},
			{ID: "geo-2", Name: "Zone B"},
			{ID: "geo-3", Name: "Zone C"},
		},
		Latitude:  19.4326,
		Longitude: -99.1332,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = res.ContainsGeofence("Zone B")
		_ = res.GeofenceIDs()
		_ = res.GeofenceNames()
	}
}
