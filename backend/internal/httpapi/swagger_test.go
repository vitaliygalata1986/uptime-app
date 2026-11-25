package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestSwaggerContract(t *testing.T) {
	a := localAPI()
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("contract: %d %s", w.Code, w.Body)
	}
	var doc struct {
		Swagger string `json:"swagger"`
		Paths   map[string]map[string]struct {
			Security []map[string][]string `json:"security"`
		} `json:"paths"`
		Definitions map[string]struct {
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Swagger != "2.0" {
		t.Fatalf("unexpected specification version: %q", doc.Swagger)
	}
	// Compare the served contract with the checked-in artifact and the actual route table.
	artifact, err := os.ReadFile("../../docs/swagger.json")
	if err != nil {
		t.Fatal(err)
	}
	var served, generated map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &served); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(artifact, &generated); err != nil {
		t.Fatal(err)
	}
	// Swag's runtime template includes empty server overrides that the JSON artifact omits.
	if served["host"] == "" {
		delete(served, "host")
	}
	if schemes, ok := served["schemes"].([]any); ok && len(schemes) == 0 {
		delete(served, "schemes")
	}
	if !reflect.DeepEqual(served, generated) {
		t.Fatal("served contract differs from docs/swagger.json")
	}
	expected := make(map[string]map[string]bool)
	for path, methods := range a.routes {
		expected[path] = make(map[string]bool)
		for method := range methods {
			expected[path][strings.ToLower(method)] = true
		}
	}
	expected["/uploads/avatars/{file}"] = map[string]bool{"get": true}
	expected["/uploads/files/{file}"] = map[string]bool{"get": true}
	if len(doc.Paths) != len(expected) {
		t.Fatalf("documented paths: %d, actual: %d", len(doc.Paths), len(expected))
	}
	for path, methods := range expected {
		if len(doc.Paths[path]) != len(methods) {
			t.Errorf("documented methods differ for %s", path)
		}
		for method := range methods {
			operation, ok := doc.Paths[path][method]
			if !ok {
				t.Errorf("missing operation: %s %s", method, path)
				continue
			}
			protected := path == "/api/auth/me" || path == "/api/auth/profile" || path == "/api/auth/avatar" ||
				path == "/api/uploads" || path == "/api/monitors" || path == monitorItemRoute
			var bearer bool
			if len(operation.Security) == 1 {
				_, bearer = operation.Security[0]["BearerAuth"]
			}
			if bearer != protected {
				t.Errorf("incorrect Bearer security: %s %s", method, path)
			}
		}
	}
	properties := doc.Definitions["auth.Result"].Properties
	if len(properties) != 4 || properties["refresh"] != nil || properties["expires_at"] != nil {
		t.Fatal("auth contract must contain public JSON fields only")
	}
	if doc.Definitions["auth.User"].Properties["password_hash"] != nil {
		t.Fatal("password hash exposed in the contract")
	}
}

func TestSwaggerAssetsAndGuards(t *testing.T) {
	for _, path := range []string{"/swagger/index.html", "/swagger/swagger-ui.css", "/swagger/swagger-ui-bundle.js"} {
		w := httptest.NewRecorder()
		localAPI().ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK || w.Body.Len() == 0 {
			t.Errorf("asset %s: %d", path, w.Code)
		}
	}
	for _, tc := range []struct {
		method, origin string
		status         int
	}{
		{http.MethodPost, "http://localhost:3000", http.StatusMethodNotAllowed},
		{http.MethodGet, "https://evil.example", http.StatusForbidden},
	} {
		r := httptest.NewRequest(tc.method, "/swagger/doc.json", nil)
		r.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		localAPI().ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Errorf("%s origin %s: %d", tc.method, tc.origin, w.Code)
		}
	}
}
