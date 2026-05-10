# NexPay SDK

一个基于 Go 的统一支付 SDK 服务，封装 **微信支付 v3** 与 **支付宝**，对外提供
RESTful HTTP API，订单、退款、回调流水统一持久化到 **MySQL**。

## 目录结构

```
.
├── cmd/server/            服务启动入口（main）
├── configs/               配置文件目录（config.yaml）
├── internal/
│   ├── api/               Gin 路由 & Handler
│   ├── config/            配置加载（viper）
│   ├── model/             GORM 数据模型
│   ├── repository/        MySQL 仓储层
│   ├── service/           业务编排（统一支付服务）
│   └── payment/           支付渠道抽象
│       ├── wechat/        微信支付 v3 封装
│       └── alipay/        支付宝封装
├── pkg/
│   ├── logger/            zap 日志
│   └── response/          统一响应体
├── migrations/            可选 SQL 初始化脚本
└── Makefile
```

## 快速开始

### 1. 准备 MySQL

请先确保可访问的 MySQL 已启动，并把连接信息写入 `configs/config.yaml` 的 `mysql` 段。

### 2. 配置渠道密钥

编辑 `configs/config.yaml`：

```yaml
wechat:
  enabled: true
  app_id: "wx_xxx"
  mch_id: "1900xxxxxx"
  serial_no: "xxx"
  api_v3_key: "xxx_32_chars"
  private_key_path: "configs/wechat/apiclient_key.pem"

alipay:
  enabled: true
  app_id: "20210xxxxxxxxxxx"
  private_key: "MIIEvQIBADANB..."     # 应用私钥（PKCS8 推荐）
  ali_public_key: "MIIBIjANBgkq..."   # 支付宝公钥
  is_production: false
```

> 也可以使用环境变量覆盖任意配置项，例如：
> `NEXPAY_MYSQL__PASSWORD=xxx NEXPAY_WECHAT__ENABLED=true ./bin/nexpay`

API 鉴权（可选，默认关闭）：

```yaml
auth:
  enabled: true
  header: "X-API-Key"
  api_keys:
    - "replace_with_strong_api_key"
```

开启后，`/api/v1/payments`、`/api/v1/payments/:out_trade_no`、`/api/v1/refunds` 需携带
`X-API-Key`（或 `Authorization: Bearer <key>`）。

鉴权请求头示例：

```bash
# 方式一：X-API-Key
curl -H "X-API-Key: ${API_KEY}" http://localhost:8080/api/v1/payments/T20260510001

# 方式二：Authorization Bearer
curl -H "Authorization: Bearer ${API_KEY}" http://localhost:8080/api/v1/payments/T20260510001
```

日志文件滚动配置示例（已集成 lumberjack）：

```yaml
log:
  level: "info"
  encoding: "json"
  output_paths: ["stdout"]
  file_path: "./logs/nexpay.log"
  max_size_mb: 100
  max_backups: 7
  max_age_days: 30
  compress: true
```

### 3. 初始化数据库（仅首次 / 表结构变更时）

```bash
make tidy
make migrate                 # 等价于 ./bin/nexpay migrate --config configs/config.yaml
```

这会创建/迁移 3 张表：`orders`、`refunds`、`payment_notify_logs`。
也可直接 `mysql -u... < migrations/001_init.sql`，二者结果一致。

> **重要**：服务启动不再自动跑迁移，避免每次启动都对远程 MySQL 执行 DDL。

### 4. 启动服务

```bash
make run                 # 开发态：go run
# 或
make start               # 生产态：编译后二进制启动
# 看到 "server started addr=0.0.0.0:8080" 即启动成功
```

子命令一览：

```bash
nexpay              # 默认启动 HTTP 服务
nexpay --config configs/config.yaml  # 启动 HTTP 服务
nexpay migrate      # 仅执行数据库迁移后退出（别名：-m / --migrate）
nexpay version      # 打印版本信息（别名：-v / --version）
nexpay help         # 打印帮助（别名：-h / --help）
```

## API 一览

| 方法 | 路径                                    | 说明           |
|------|-----------------------------------------|----------------|
| GET  | `/healthz`                              | 健康检查       |
| POST | `/api/v1/payments`                      | 统一下单       |
| GET  | `/api/v1/payments/:out_trade_no`        | 查询订单       |
| POST | `/api/v1/refunds`                       | 申请退款       |
| POST | `/api/v1/notify/wechat`                 | 微信支付回调   |
| POST | `/api/v1/notify/alipay`                 | 支付宝回调     |
| POST | `/api/v1/notify/wechat/refund`          | 微信退款回调   |
| POST | `/api/v1/notify/alipay/refund`          | 支付宝退款回调 |

### 统一下单示例

> 若已开启 `auth.enabled: true`，先执行：`export API_KEY='replace_with_strong_api_key'`

#### 微信扫码支付（Native）

```bash
curl -X POST http://localhost:8080/api/v1/payments \
  -H 'Content-Type: application/json' \
  -H "X-API-Key: ${API_KEY}" \
  -d '{
    "channel": "wechat",
    "trade_type": "NATIVE",
    "out_trade_no": "T20260510001",
    "subject": "测试商品",
    "amount": 1
  }'
```

返回：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "order": { "id": 1, "out_trade_no": "T20260510001", "status": "PENDING", "...": "..." },
    "pay":   { "channel": "wechat", "qr_code_url": "weixin://wxpay/bizpayurl?pr=xxx" }
  }
}
```

#### 支付宝 PC 网页支付（PAGE）

```bash
curl -X POST http://localhost:8080/api/v1/payments \
  -H 'Content-Type: application/json' \
  -H "X-API-Key: ${API_KEY}" \
  -d '{
    "channel": "alipay",
    "trade_type": "PAGE",
    "out_trade_no": "T20260510002",
    "subject": "测试商品",
    "amount": 100
  }'
```

返回 `data.pay.pay_url`，前端直接重定向即可。

### 支持的 trade_type

| 渠道   | trade_type                              |
|--------|-----------------------------------------|
| wechat | `NATIVE` / `JSAPI` / `APP` / `H5`       |
| alipay | `PAGE` / `WAP` / `APP` / `QR`           |

> 微信 `JSAPI` 必须额外传 `payer_open_id`。

### 查询订单

```bash
curl -H "X-API-Key: ${API_KEY}" \
  http://localhost:8080/api/v1/payments/T20260510001
```

服务会先调用渠道接口查最新状态，差异时同步更新本地订单。

### 退款

```bash
curl -X POST http://localhost:8080/api/v1/refunds \
  -H 'Content-Type: application/json' \
  -H "X-API-Key: ${API_KEY}" \
  -d '{
    "out_trade_no": "T20260510001",
    "out_refund_no": "R20260510001",
    "refund_amount": 1,
    "reason": "用户申请退款"
  }'
```

## 异步通知

- `notify.base_url` + 各渠道 `notify_path/refund_notify_path` 必须是公网可达地址
- 服务自动完成 **签名校验** + **金额校验** + **流水入库** + **订单状态更新**
- 校验失败返回 4xx，渠道会按其策略重试
- 校验成功后给微信返回 `{"code":"SUCCESS"}`，给支付宝返回 `success`

### 回调地址拼接示例

```yaml
notify:
  base_url: "https://pay.example.com"

wechat:
  notify_path: "/api/v1/notify/wechat"
  refund_notify_path: "/api/v1/notify/wechat/refund"

alipay:
  notify_path: "/api/v1/notify/alipay"
  refund_notify_path: "/api/v1/notify/alipay/refund"
```

最终给平台配置的回调地址为：

- 微信支付回调：`https://pay.example.com/api/v1/notify/wechat`
- 微信退款回调：`https://pay.example.com/api/v1/notify/wechat/refund`
- 支付宝支付回调：`https://pay.example.com/api/v1/notify/alipay`
- 支付宝退款回调：`https://pay.example.com/api/v1/notify/alipay/refund`

### 回调返回示例

微信（支付/退款）成功应答：

```json
{"code":"SUCCESS","message":"OK"}
```

支付宝（支付/退款）成功应答：

```text
success
```

### 支付宝退款回调示例（表单）

> 下面仅用于展示字段形态，真实请求需由支付宝服务器发起并带签名参数。

```bash
curl -X POST "https://pay.example.com/api/v1/notify/alipay/refund" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-raw "notify_type=trade_status_sync&out_trade_no=T20260510001&out_biz_no=R20260510001&trade_no=2026051022001400001234567890&refund_status=REFUND_SUCCESS&refund_fee=0.01&sign=xxxx&sign_type=RSA2"
```

### 本地联调建议

- 使用 `ngrok` / `frp` 暴露本地地址，把公网域名填到 `notify.base_url`
- 确保平台后台配置的回调路径与 `notify_path/refund_notify_path` 一致
- 回调后可在 `payment_notify_logs` 表查看原始通知和验签结果

## 数据模型

| 表                     | 说明              |
|------------------------|-------------------|
| `orders`               | 业务订单          |
| `refunds`              | 退款单            |
| `payment_notify_logs`  | 异步通知审计流水  |

字段定义见 [`internal/model/order.go`](internal/model/order.go)。

## 扩展新渠道

1. 在 `internal/payment/<channel>/` 下新建 Provider，实现 `payment.Provider` 接口
2. 在 `cmd/server/main.go` 的 `buildProviders` 中注册
3. 在 `internal/api/handler.go` / `router.go` 中新增对应的 notify handler（如需）

## 注意事项

- 生产环境请把 `server.mode` 改为 `release`
