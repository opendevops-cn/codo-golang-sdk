# codo golang SDK

## 背景
许多的业务需求其实有大量的重复工作, 可以使用通用的能力完成

## 目标
- [ ] 基础架构
  - [ ] 通用的 mysql 客户端支持 otel
  - [ ] 通用的 redis 客户端支持 otel
  - [ ] 通用的 http 客户端支持 otel
  - [ ] 通用的 消息队列 otel
  - [ ] 通用的 协程池 支持 otel
  - [ ] 通用的 配置 组件
  - [ ] 通用的 日志 组件
  - [ ] 通用的 otel propagator
- [x] **OpenAPI 客户端**（AK/SK · `CODO1-HMAC-SHA256`）→ [`openapi/`](./openapi/)
- [ ] 工具链
  - [ ] 自动生成 HTTP 代码(proto 转 http)
  - [ ] 自动生成 MYSQL 代码(数据库转结构体)
  - [ ] LINT 检查

## OpenAPI（AK/SK 调网关）

程序 / CI 用 **API 凭证** 调 Codo 业务 API 时，使用子包 `openapi`：

```go
import "github.com/opendevops-cn/codo-golang-sdk/openapi"

client := openapi.NewClient("<AK>", "<SK>", "https://gw.example.com")
resp, err := client.Get(ctx, "/api/p/v4/biz/list/", nil)
```

- 协议：`CODO1-HMAC-SHA256`（与 Python `websdk2.openapi_client`、网关 `openapi_aksk` 对齐）
- **不是** `middleware` / `tools/xsgin` 的 MD5 `x-sign`
- 说明、向量与排错：[openapi/README.md](./openapi/README.md)

```bash
go test ./openapi/ -count=1
```

## 目录结构
```
.
├── CHANGELOG.md 变更日志
├── Makefile 快捷工具
├── README.md
├── adapter 适配器
│   └── kratos kratos 适配
├── app 应用层
├── client 客户端
│   └── xhttp http 客户端
├── config 配置统一处理
│   ├── config.go
│   ├── config_test.go
│   └── testdata
├── consts 常量定义
│   ├── bytes.go
│   └── consts.go
├── go.mod
├── go.sum
├── internal # 私有包
│   └── meta # lib元数据
├── logger # 日志组件
├── middleware # 通用中间件（含旧版 MD5 x-sign，勿与 OpenAPI 混淆）
├── mysql
├── openapi # Codo OpenAPI AK/SK 客户端（CODO1-HMAC-SHA256）
│   ├── sign.go
│   ├── client.go
│   └── README.md
├── redis
├── tools
│   ├── cascmd
│   └── xsgin # 旧 MD5 签名
└── xnet
```
