package openapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestClientGetSignsHeaders(t *testing.T) {
	var got http.Header
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":0}`))
	}))
	defer srv.Close()

	c := NewClient(testAK, testSK, srv.URL)
	resp, err := c.Do(context.Background(), http.MethodGet, "/api/p/v4/biz/list/", &DoOptions{
		Auth: &AuthHeaderOptions{Timestamp: testTS, Nonce: testN},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if gotPath != "/api/p/v4/biz/list/" {
		t.Fatalf("path %s", gotPath)
	}
	if got.Get(HeaderSignature) != wantGET {
		t.Fatalf("sig got %s want %s", got.Get(HeaderSignature), wantGET)
	}
	if got.Get(HeaderAccessKey) != testAK {
		t.Fatalf("ak %s", got.Get(HeaderAccessKey))
	}
}

func TestClientPostJSONSignsAndBody(t *testing.T) {
	var gotSig, gotCT string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get(HeaderSignature)
		gotCT = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(testAK, testSK, srv.URL)
	// Use Do with fixed auth so we can assert against the POST vector.
	body := []byte(`{"page":1}`)
	resp, err := c.Do(context.Background(), http.MethodPost, "/api/p/v4/user/", &DoOptions{
		Body:        body,
		ContentType: "application/json",
		Auth:        &AuthHeaderOptions{Timestamp: testTS, Nonce: testN, ContentType: "application/json"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if gotSig != wantPOST {
		t.Fatalf("sig got %s want %s", gotSig, wantPOST)
	}
	if gotCT != "application/json" {
		t.Fatalf("ct %s", gotCT)
	}
	if string(gotBody) != `{"page":1}` {
		t.Fatalf("body %q", gotBody)
	}
}

func TestMarshalJSONCompact(t *testing.T) {
	b, err := marshalJSONCompact(map[string]any{"page": 1})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"page":1}` {
		t.Fatalf("got %q", b)
	}
	// round-trip
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
}

func TestClientGetWithQuery(t *testing.T) {
	var rawQuery string
	var gotSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		gotSig = r.Header.Get(HeaderSignature)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	q := url.Values{}
	q.Set("page", "1")
	c := NewClient(testAK, testSK, srv.URL)
	resp, err := c.Get(context.Background(), "/api/p/v4/biz/list/", q)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if rawQuery != "page=1" {
		t.Fatalf("query %q", rawQuery)
	}
	if gotSig == "" {
		t.Fatal("missing signature")
	}
}

func TestResolveAbsoluteURL(t *testing.T) {
	c := NewClient(testAK, testSK, "https://unused.example")
	path, query, base, err := c.resolveURL("https://gw.example.com/api/p/v4/x?a=1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if path != "/api/p/v4/x" || base != "https://gw.example.com" || query.Get("a") != "1" {
		t.Fatalf("path=%s base=%s query=%v", path, base, query)
	}
}
