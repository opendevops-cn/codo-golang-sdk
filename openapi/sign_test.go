package openapi

import (
	"net/url"
	"testing"
)

// Fixed vectors from docs/openapi-access.md and websdk2.openapi_sign tests.
const (
	testAK = "codoAKTEST0001"
	testSK = "testSecretKeyForOpenAPISign0001"
	testTS = "1700000000"
	testN  = "nonce-fixed-001"

	wantGET  = "ce94e413a8f752ed75a7c129c184f14892d8b24f30222607994276174339e07f"
	wantPOST = "4a2adef69ff58b92bc99c04474811e78082a5e3f5675a3c1d26e19ed3e7dc163"
)

func TestSignGETVector(t *testing.T) {
	headers := map[string]string{
		"x-codo-access-key": testAK,
		"x-codo-timestamp":  testTS,
		"x-codo-nonce":      testN,
	}
	got := Sign(testSK, "GET", "/api/p/v4/biz/list/", nil, headers, nil, testTS)
	if got != wantGET {
		t.Fatalf("GET signature\n got %s\nwant %s", got, wantGET)
	}
}

func TestSignPOSTVector(t *testing.T) {
	headers := map[string]string{
		"x-codo-access-key": testAK,
		"x-codo-timestamp":  testTS,
		"x-codo-nonce":      testN,
		"content-type":      "application/json",
	}
	body := []byte(`{"page":1}`)
	got := Sign(testSK, "POST", "/api/p/v4/user/", nil, headers, body, testTS)
	if got != wantPOST {
		t.Fatalf("POST signature\n got %s\nwant %s", got, wantPOST)
	}
}

func TestBuildAuthHeadersGET(t *testing.T) {
	h, err := BuildAuthHeaders(testAK, testSK, "GET", "/api/p/v4/biz/list/", nil, nil, &AuthHeaderOptions{
		Timestamp: testTS,
		Nonce:     testN,
	})
	if err != nil {
		t.Fatal(err)
	}
	if h[HeaderSignature] != wantGET {
		t.Fatalf("auth GET sig\n got %s\nwant %s", h[HeaderSignature], wantGET)
	}
	if h[HeaderAccessKey] != testAK || h[HeaderTimestamp] != testTS || h[HeaderNonce] != testN {
		t.Fatalf("unexpected headers: %+v", h)
	}
}

func TestBuildAuthHeadersPOST(t *testing.T) {
	body := []byte(`{"page":1}`)
	h, err := BuildAuthHeaders(testAK, testSK, "POST", "/api/p/v4/user/", nil, body, &AuthHeaderOptions{
		Timestamp:   testTS,
		Nonce:       testN,
		ContentType: "application/json",
	})
	if err != nil {
		t.Fatal(err)
	}
	if h[HeaderSignature] != wantPOST {
		t.Fatalf("auth POST sig\n got %s\nwant %s", h[HeaderSignature], wantPOST)
	}
	if h["Content-Type"] != "application/json" {
		t.Fatalf("content-type missing: %+v", h)
	}
}

func TestCanonicalQueryEncoding(t *testing.T) {
	q := url.Values{}
	q.Set("b", "2")
	q.Set("a", "1 2")
	q.Add("a", "0")
	// keys sorted: a, b; values of a sorted: "0", "1 2"
	got := CanonicalQuery(q)
	want := "a=0&a=1%202&b=2"
	if got != want {
		t.Fatalf("query\n got %q\nwant %q", got, want)
	}
}

func TestEmptyBodyHash(t *testing.T) {
	// SHA256 of empty bytes — used as last line of canonical request when no body.
	const emptyHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if SHA256Hex(nil) != emptyHash {
		t.Fatalf("nil body hash = %s", SHA256Hex(nil))
	}
	if SHA256Hex([]byte{}) != emptyHash {
		t.Fatalf("empty body hash = %s", SHA256Hex([]byte{}))
	}
}
