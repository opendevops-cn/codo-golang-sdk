# openapi — Codo OpenAPI Go SDK

机器身份调用 Codo 网关业务 API：用 **AK/SK** 对每次请求做 **`CODO1-HMAC-SHA256`** 签名。

与本仓库其它签名能力无关：

| 包 | 协议 | 用途 |
|----|------|------|
| **`openapi`（本包）** | CODO1-HMAC-SHA256，头 `X-Codo-*` | 开放接入 · API 凭证 |
| `tools/xsgin` + `middleware` | MD5，`x-sign` query | 旧业务中间件 |
| `idip/signer` | MD5，`cbb-sign-*` | IDIP |

对齐实现：Python `websdk2.openapi_sign` / `openapi_client`，网关 `openapi_aksk` 插件。

## 安装

```bash
go get github.com/opendevops-cn/codo-golang-sdk/openapi@latest
```

模块路径：`github.com/opendevops-cn/codo-golang-sdk`（子包 `openapi`）。

## 快速开始

```go
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/opendevops-cn/codo-golang-sdk/openapi"
)

func main() {
	client := openapi.NewClient(
		os.Getenv("CODO_AK"),
		os.Getenv("CODO_SK"),
		"https://gw.example.com", // 网关根，不要尾斜杠
	)

	resp, err := client.Get(context.Background(), "/api/p/v4/biz/list/", nil)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Println(resp.StatusCode, string(body))
}
```

POST JSON：

```go
resp, err := client.PostJSON(context.Background(), "/api/p/v4/user/", map[string]any{
	"page": 1,
}, nil)
```

只签不发（自管 HTTP）：

```go
headers, err := openapi.BuildAuthHeaders(ak, sk, "GET", "/api/p/v4/biz/list/", nil, nil, nil)
// headers["X-Codo-Access-Key"] / Timestamp / Nonce / Signature
```

## 请求头

| 头 | 含义 |
|----|------|
| `X-Codo-Access-Key` | AK |
| `X-Codo-Timestamp` | Unix 秒，与服务器差建议 ≤ 300s |
| `X-Codo-Nonce` | 随机串，同 AK 短时勿重复 |
| `X-Codo-Signature` | 签名，小写 hex |
| `Content-Type` | 有 body 时用 `application/json`，**并参与签名** |

## 签名摘要

1. `PayloadHash = Hex(SHA256(body))`；无 body 时对**空字节**做 SHA256  
2. CanonicalRequest 六行：`METHOD`、`path`、规范化 query、规范化头、signed headers、PayloadHash  
3. `StringToSign = CODO1-HMAC-SHA256\n{ts}\nHex(SHA256(CanonicalRequest))`  
4. `Signature = Hex(HMAC-SHA256(SK, StringToSign))`

完整规范与 curl 手写步骤见 Codo 管理端文档：`docs/openapi-access.md`。

### 固定测试向量（单测锁死）

| 场景 | Signature |
|------|-----------|
| GET `/api/p/v4/biz/list/`，空 body，AK/SK/ts/nonce 见下 | `ce94e413a8f752ed75a7c129c184f14892d8b24f30222607994276174339e07f` |
| POST `/api/p/v4/user/`，body `{"page":1}`，`Content-Type: application/json` | `4a2adef69ff58b92bc99c04474811e78082a5e3f5675a3c1d26e19ed3e7dc163` |

```
AK=codoAKTEST0001
SK=testSecretKeyForOpenAPISign0001
Timestamp=1700000000
Nonce=nonce-fixed-001
```

```bash
go test ./openapi/ -count=1
```

## API 一览

| 符号 | 说明 |
|------|------|
| `NewClient(ak, sk, endpoint, opts...)` | 构造客户端；`WithHTTPClient` / `WithTimeout` |
| `(*Client).Get / Delete / PostJSON / PutJSON / Request / Do` | 签名并发请求，返回 `*http.Response` |
| `BuildAuthHeaders` | 仅生成签名头 |
| `Sign` / `BuildCanonicalRequest` / `BuildStringToSign` | 底层签名（排错 / 自定义传输） |

## 排错

| 现象 | 先查 |
|------|------|
| **401** | AK/SK、时钟差、nonce 复用、path 是否与网关一致、body 是否与签名用同一字节 |
| **403** | 账号角色是否包含该 API（不是签名问题） |

SK 只放服务端；每请求重新签名，不要缓存旧 `X-Codo-Signature`。

## 与旧模块路径

历史文档曾写 `github.com/ss1917/codo-openapi-go/openapi`（未正式入库）。  
请改用：

```go
import "github.com/opendevops-cn/codo-golang-sdk/openapi"
```
