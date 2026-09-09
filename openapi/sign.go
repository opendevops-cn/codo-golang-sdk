// Package openapi provides CODO OpenAPI (AK/SK) signing and an HTTP client.
//
// Algorithm: CODO1-HMAC-SHA256 — aligned with websdk2.openapi_sign and the
// gateway openapi_aksk plugin. Unrelated to tools/xsgin (MD5) or idip/signer.
package openapi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// AlgorithmName is the fixed algorithm line in StringToSign.
	AlgorithmName = "CODO1-HMAC-SHA256"

	HeaderAccessKey = "X-Codo-Access-Key"
	HeaderTimestamp = "X-Codo-Timestamp"
	HeaderNonce     = "X-Codo-Nonce"
	HeaderSignature = "X-Codo-Signature"
)

var hexUpper = "0123456789ABCDEF"

// SHA256Hex returns lowercase hex SHA-256 of data. A nil body is treated as empty.
func SHA256Hex(data []byte) string {
	if data == nil {
		data = []byte{}
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// HMACSHA256Hex returns lowercase hex HMAC-SHA256(key, msg).
func HMACSHA256Hex(key, msg string) string {
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

// encodeRFC3986 percent-encodes s, keeping unreserved chars: ALPHA / DIGIT / - . _ ~
// (same as Python urllib.parse.quote(..., safe='-_.~')).
func encodeRFC3986(s string) string {
	var b strings.Builder
	b.Grow(len(s) * 2)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '.' || c == '_' || c == '~' {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hexUpper[c>>4])
		b.WriteByte(hexUpper[c&0x0f])
	}
	return b.String()
}

// CanonicalQuery builds a sorted, RFC3986-encoded query string.
// Empty or nil → "". Multi-value keys sort their values.
func CanonicalQuery(params url.Values) string {
	if len(params) == 0 {
		return ""
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(params))
	for _, k := range keys {
		vs := params[k]
		if len(vs) == 0 {
			continue
		}
		sorted := append([]string(nil), vs...)
		sort.Strings(sorted)
		ek := encodeRFC3986(k)
		for _, v := range sorted {
			parts = append(parts, ek+"="+encodeRFC3986(v))
		}
	}
	return strings.Join(parts, "&")
}

// CanonicalQueryMap is a convenience for simple string maps.
func CanonicalQueryMap(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}
	v := make(url.Values, len(params))
	for k, val := range params {
		v.Set(k, val)
	}
	return CanonicalQuery(v)
}

// BuildCanonicalRequest builds the 6-line canonical request.
// Header names are lower-cased and sorted; values are trimmed.
func BuildCanonicalRequest(method, uri string, query url.Values, headers map[string]string, body []byte) string {
	if body == nil {
		body = []byte{}
	}
	normalized := make(map[string]string, len(headers))
	for k, v := range headers {
		normalized[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
	}
	names := make([]string, 0, len(normalized))
	for n := range normalized {
		names = append(names, n)
	}
	sort.Strings(names)

	var ch strings.Builder
	for _, n := range names {
		ch.WriteString(n)
		ch.WriteByte(':')
		ch.WriteString(normalized[n])
		ch.WriteByte('\n')
	}
	signed := strings.Join(names, ";")

	return strings.Join([]string{
		strings.ToUpper(method),
		uri,
		CanonicalQuery(query),
		ch.String(),
		signed,
		SHA256Hex(body),
	}, "\n")
}

// BuildStringToSign builds StringToSign for CODO1-HMAC-SHA256.
func BuildStringToSign(timestamp, canonicalRequest string) string {
	return strings.Join([]string{
		AlgorithmName,
		timestamp,
		SHA256Hex([]byte(canonicalRequest)),
	}, "\n")
}

// Sign computes the request signature (lowercase hex).
func Sign(secretKey, method, uri string, query url.Values, headers map[string]string, body []byte, timestamp string) string {
	cr := BuildCanonicalRequest(method, uri, query, headers, body)
	sts := BuildStringToSign(timestamp, cr)
	return HMACSHA256Hex(secretKey, sts)
}

// AuthHeaderOptions controls BuildAuthHeaders.
type AuthHeaderOptions struct {
	// Timestamp is unix seconds as decimal string; empty uses time.Now().
	Timestamp string
	// Nonce; empty generates 8 random bytes as hex (16 chars, same width as Python token_hex(8)).
	Nonce string
	// ContentType is set on the returned headers. It is included in the signed
	// header set only when body is non-empty (Python parity).
	ContentType string
}

// BuildAuthHeaders returns X-Codo-* auth headers (and Content-Type when set).
func BuildAuthHeaders(
	accessKey, secretKey, method, uri string,
	query url.Values,
	body []byte,
	opt *AuthHeaderOptions,
) (map[string]string, error) {
	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("openapi: access_key and secret_key are required")
	}
	if method == "" {
		return nil, fmt.Errorf("openapi: method is required")
	}
	if uri == "" {
		uri = "/"
	}

	var ts, nonce, contentType string
	if opt != nil {
		ts = opt.Timestamp
		nonce = opt.Nonce
		contentType = opt.ContentType
	}
	if ts == "" {
		ts = strconv.FormatInt(time.Now().Unix(), 10)
	}
	if nonce == "" {
		var b [8]byte
		if _, err := rand.Read(b[:]); err != nil {
			return nil, fmt.Errorf("openapi: nonce: %w", err)
		}
		nonce = hex.EncodeToString(b[:])
	}
	if body == nil {
		body = []byte{}
	}

	signHeaders := map[string]string{
		"x-codo-access-key": accessKey,
		"x-codo-timestamp":  ts,
		"x-codo-nonce":      nonce,
	}
	if contentType != "" && len(body) > 0 {
		signHeaders["content-type"] = contentType
	}

	sig := Sign(secretKey, method, uri, query, signHeaders, body, ts)

	out := map[string]string{
		HeaderAccessKey: accessKey,
		HeaderTimestamp: ts,
		HeaderNonce:     nonce,
		HeaderSignature: sig,
	}
	if contentType != "" {
		out["Content-Type"] = contentType
	}
	return out, nil
}
