package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// unreadyStore es un appStore de prueba cuyo Ready() es false.
type unreadyStore struct {
	*memStore
}

func (u *unreadyStore) Ready() bool { return false }

func TestSwaggerEndpoints(t *testing.T) {
	st := newMemStore(false)
	handler := gate(st, appMux(st))

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantType   string
		wantBody   string
	}{
		{
			name:       "Swagger UI direct path",
			path:       "/swagger",
			wantStatus: http.StatusOK,
			wantType:   "text/html; charset=utf-8",
			wantBody:   `<div id="swagger-ui"></div>`,
		},
		{
			name:       "Swagger UI trailing slash",
			path:       "/swagger/",
			wantStatus: http.StatusOK,
			wantType:   "text/html; charset=utf-8",
			wantBody:   `<div id="swagger-ui"></div>`,
		},
		{
			name:       "Docs redirect to /swagger",
			path:       "/docs",
			wantStatus: http.StatusMovedPermanently,
			wantType:   "text/html; charset=utf-8",
			wantBody:   `/swagger`,
		},
		{
			name:       "OpenAPI json spec",
			path:       "/openapi.json",
			wantStatus: http.StatusOK,
			wantType:   "application/json",
			wantBody:   `"openapi": "3.0.3"`,
		},
		{
			name:       "Root service info",
			path:       "/",
			wantStatus: http.StatusOK,
			wantType:   "application/json",
			wantBody:   `"service":"target-app"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("path %s status=%d, want %d", tc.path, w.Code, tc.wantStatus)
			}
			if tc.wantType != "" && !strings.HasPrefix(w.Header().Get("Content-Type"), tc.wantType) {
				t.Errorf("path %s Content-Type=%q, want prefix %q", tc.path, w.Header().Get("Content-Type"), tc.wantType)
			}
			if tc.wantBody != "" && !strings.Contains(w.Body.String(), tc.wantBody) {
				t.Errorf("path %s body missing substring %q\ngot:\n%s", tc.path, tc.wantBody, w.Body.String())
			}
		})
	}
}

func TestGateBypassesStaticEndpointsWhenDBUnready(t *testing.T) {
	st := &unreadyStore{memStore: newMemStore(false)}
	handler := gate(st, appMux(st))

	// Endpoints que deben responder aunque la base este caida
	allowed := []string{"/", "/openapi.json", "/swagger", "/swagger/", "/docs"}
	for _, p := range allowed {
		req := httptest.NewRequest("GET", p, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code == http.StatusServiceUnavailable {
			t.Errorf("gate should allow %s when DB is not ready, got 503", p)
		}
	}

	// Endpoints que dependen de la base deben dar 503
	blocked := []string{"/health", "/products", "/orders"}
	for _, p := range blocked {
		req := httptest.NewRequest("GET", p, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("gate should block %s when DB is not ready, got %d", p, w.Code)
		}
	}
}

func TestOpenAPISpecValidity(t *testing.T) {
	var spec struct {
		OpenAPI string `json:"openapi"`
		Info    struct {
			Title   string `json:"title"`
			Version string `json:"version"`
		} `json:"info"`
		Paths map[string]map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(openAPIDoc, &spec); err != nil {
		t.Fatalf("openapi.json no es JSON valido: %v", err)
	}
	if spec.OpenAPI != "3.0.3" {
		t.Errorf("openapi version=%q, want 3.0.3", spec.OpenAPI)
	}
	expectedPaths := []string{"/", "/health", "/products", "/orders"}
	for _, p := range expectedPaths {
		if _, ok := spec.Paths[p]; !ok {
			t.Errorf("falta path en openapi.json: %s", p)
		}
	}
}

func TestProductsAndOrdersInMemStore(t *testing.T) {
	st := newMemStore(false)
	ctx := context.Background()

	// List products inicial (seed: cafe: 10, te: 5)
	ps, err := st.ListProducts(ctx)
	if err != nil {
		t.Fatalf("ListProducts: %v", err)
	}
	if len(ps) != 2 {
		t.Fatalf("expected 2 seed products, got %d", len(ps))
	}

	// Orden valida
	o, err := st.PlaceOrder(ctx, 1, 3)
	if err != nil {
		t.Fatalf("PlaceOrder fallo: %v", err)
	}
	if o.Qty != 3 || o.ProductID != 1 {
		t.Errorf("Order inesperada: %+v", o)
	}

	// Stock debe ser 7
	ps, _ = st.ListProducts(ctx)
	if ps[0].Stock != 7 {
		t.Errorf("Stock esperado 7, obtenido %d", ps[0].Stock)
	}

	// Orden que supera el stock con oversell=false debe fallar
	_, err = st.PlaceOrder(ctx, 1, 8)
	if err != errInsufficientStock {
		t.Errorf("expected errInsufficientStock, got %v", err)
	}
}
