package openapi_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/opendevops-cn/codo-golang-sdk/openapi"
)

func ExampleClient_Get() {
	// In production: openapi.NewClient(ak, sk, "https://gw.example.com")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println(r.Header.Get(openapi.HeaderAccessKey) != "")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":0,"msg":"ok"}`))
	}))
	defer srv.Close()

	client := openapi.NewClient("codoAKxxxx", "secret", srv.URL)
	resp, err := client.Get(context.Background(), "/api/p/v4/biz/list/", nil)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Println(resp.StatusCode, string(body))
	// Output:
	// true
	// 200 {"code":0,"msg":"ok"}
}
