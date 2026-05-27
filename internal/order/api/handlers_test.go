package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/shopspring/decimal"

	"github.com/Grandeath/order-service/internal/auth"
	"github.com/Grandeath/order-service/internal/order/api"
	"github.com/Grandeath/order-service/internal/order/domain"
	"github.com/Grandeath/order-service/internal/order/repository"
	"github.com/Grandeath/order-service/internal/order/service"
	"github.com/Grandeath/order-service/internal/server"
)

// fakeVerifier issues a token carrying a fixed subject, simulating an
// authenticated Cognito user without needing real JWKS / signatures.
type fakeVerifier struct{ sub string }

func (f fakeVerifier) Verify(_ context.Context, _ string) (jwt.Token, error) {
	return jwt.NewBuilder().Subject(f.sub).Build()
}

func newTestServer(t *testing.T) (http.Handler, *service.Service) {
	t.Helper()
	repo := repository.NewInMemory()
	idCounter := 0
	svc := service.New(repo,
		service.WithClock(func() time.Time {
			return time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
		}),
		service.WithIDGen(func() string {
			idCounter++
			return "ord-test-" + string(rune('0'+idCounter))
		}),
	)
	h := api.NewHandler(svc)
	// Cognito-disabled mode: customer id is resolved from request params.
	return server.Router(h.Endpoints(), h.Middlewares(auth.DevCustomerMiddleware())), svc
}

const testCustomerID = "cust-1"

func validCreateBody() any {
	return map[string]any{
		"currency": "PLN",
		"items": []map[string]any{
			{
				"productId":   "p1",
				"productName": "Product 1",
				"quantity":    2,
				"unitPrice":   "9.99",
			},
		},
		"deliveryAddress": map[string]string{
			"street":     "Marszałkowska 1",
			"city":       "Warszawa",
			"postalCode": "00-001",
			"country":    "PL",
		},
	}
}

// createReq builds a POST /orders request carrying the customer id as a query
// param, matching how DevCustomerMiddleware resolves identity in tests.
func createReq(t *testing.T, body any) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders?"+auth.CustomerIDParam+"="+testCustomerID, mustJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func mustJSON(t *testing.T, body any) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return bytes.NewBuffer(b)
}

func TestCreate_Created(t *testing.T) {
	h, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, createReq(t, validCreateBody()))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/api/v1/orders/") {
		t.Errorf("Location header = %q, want prefix /api/v1/orders/", loc)
	}

	var resp api.OrderResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != domain.OrderStatusNew {
		t.Errorf("status = %s, want NEW", resp.Status)
	}
	if resp.CustomerID != testCustomerID {
		t.Errorf("customerId = %q, want %q (from param)", resp.CustomerID, testCustomerID)
	}
	wantTotal := decimal.NewFromFloat(9.99).Mul(decimal.NewFromInt(2))
	if !resp.TotalAmount.Equal(wantTotal) {
		t.Errorf("total = %s, want %s", resp.TotalAmount, wantTotal)
	}
}

func TestCreate_CognitoEnabled_UsesJWTSubject(t *testing.T) {
	repo := repository.NewInMemory()
	svc := service.New(repo,
		service.WithClock(func() time.Time { return time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC) }),
		service.WithIDGen(func() string { return "ord-jwt-1" }),
	)
	handler := api.NewHandler(svc)
	// Cognito-enabled mode: identity comes from the verified token subject.
	h := server.Router(handler.Endpoints(), handler.Middlewares(auth.Middleware(fakeVerifier{sub: "cognito-user-42"})))

	// A customerId param is supplied but must be ignored — the JWT is authoritative.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders?customerId=spoofed", mustJSON(t, validCreateBody()))
	req.Header.Set("Authorization", "Bearer fake-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	var resp api.OrderResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.CustomerID != "cognito-user-42" {
		t.Errorf("customerId = %q, want %q (from JWT sub)", resp.CustomerID, "cognito-user-42")
	}
}

func TestCreate_CognitoDisabled_UsesParam(t *testing.T) {
	h, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, createReq(t, validCreateBody()))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	var resp api.OrderResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.CustomerID != testCustomerID {
		t.Errorf("customerId = %q, want %q (from param)", resp.CustomerID, testCustomerID)
	}
}

func TestCreate_MissingCustomerID(t *testing.T) {
	h, _ := newTestServer(t)

	// No customerId param and no JWT → empty customer → domain validation 400.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", mustJSON(t, validCreateBody()))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreate_Validation(t *testing.T) {
	h, _ := newTestServer(t)
	body := map[string]any{
		"currency": "PLN",
		"items":    []any{},
		"deliveryAddress": map[string]string{
			"street": "x", "city": "x", "postalCode": "x", "country": "x",
		},
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, createReq(t, body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreate_Idempotency(t *testing.T) {
	h, _ := newTestServer(t)

	first := createReq(t, validCreateBody())
	first.Header.Set("Idempotency-Key", "abc-123")
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, first)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("first: status = %d", rec1.Code)
	}
	var got1 api.OrderResponse
	_ = json.Unmarshal(rec1.Body.Bytes(), &got1)

	second := createReq(t, validCreateBody())
	second.Header.Set("Idempotency-Key", "abc-123")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, second)

	var got2 api.OrderResponse
	if err := json.Unmarshal(rec2.Body.Bytes(), &got2); err != nil {
		t.Fatalf("decode 2: %v", err)
	}
	if got1.ID != got2.ID {
		t.Errorf("idempotent call returned different id: %q vs %q", got1.ID, got2.ID)
	}
}

func TestGet_NotFound(t *testing.T) {
	h, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/missing", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestChangeStatus_InvalidTransition(t *testing.T) {
	h, _ := newTestServer(t)

	// Create order first.
	req := createReq(t, validCreateBody())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var created api.OrderResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	// Try to jump straight to PAID — invalid.
	patch := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/orders/"+created.ID+"/status",
		mustJSON(t, map[string]string{"status": string(domain.OrderStatusPaid)}),
	)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, patch)

	if rec2.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rec2.Code, rec2.Body.String())
	}
}

func TestChangeStatus_HappyPath(t *testing.T) {
	h, _ := newTestServer(t)

	req := createReq(t, validCreateBody())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var created api.OrderResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	patch := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/orders/"+created.ID+"/status",
		mustJSON(t, map[string]string{"status": string(domain.OrderStatusValidated)}),
	)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, patch)

	if rec2.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec2.Code, rec2.Body.String())
	}
	var got api.OrderResponse
	_ = json.Unmarshal(rec2.Body.Bytes(), &got)
	if got.Status != domain.OrderStatusValidated {
		t.Errorf("status = %s, want VALIDATED", got.Status)
	}
}

func TestCancel(t *testing.T) {
	h, _ := newTestServer(t)

	req := createReq(t, validCreateBody())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var created api.OrderResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	cancel := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/orders/"+created.ID+"/cancel",
		mustJSON(t, map[string]string{"reason": "customer changed mind"}),
	)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, cancel)

	if rec2.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec2.Code)
	}
	var got api.OrderResponse
	_ = json.Unmarshal(rec2.Body.Bytes(), &got)
	if got.Status != domain.OrderStatusCancelled {
		t.Errorf("status = %s, want CANCELLED", got.Status)
	}
}

func TestList(t *testing.T) {
	h, _ := newTestServer(t)

	for range 3 {
		req := createReq(t, validCreateBody())
		h.ServeHTTP(httptest.NewRecorder(), req)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders?customerId="+testCustomerID+"&limit=2&offset=0", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp api.ListOrdersResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Orders) != 2 {
		t.Errorf("orders = %d, want 2", len(resp.Orders))
	}
}

func TestDelete(t *testing.T) {
	h, _ := newTestServer(t)

	req := createReq(t, validCreateBody())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var created api.OrderResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	del := httptest.NewRequest(http.MethodDelete, "/api/v1/orders/"+created.ID, nil)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, del)
	if rec2.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec2.Code)
	}

	get := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+created.ID, nil)
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, get)
	if rec3.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec3.Code)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	h, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/orders", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}
