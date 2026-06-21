# Payment API

本文档聚焦项目中的支付相关接口，面向以下场景：

- 其他项目通过服务端调用支付接口
- 在其他系统中通过 agent 编排登录、下单、支付、轮询支付状态
- 需要快速接入“产品下单支付”或“直付支付”能力

所有示例默认使用版本化路径 ` /api/v1 `。

---

## 1. 基本信息

### API 根路径

生产示例：

```text
https://api2key-api.guan-afred.workers.dev/api/v1
```

Staging 示例：

```text
https://api2key-api-staging.<your-subdomain>.workers.dev/api/v1
```

### 统一响应格式

所有接口统一返回：

```json
{
  "code": 200,
  "message": "success",
  "data": {}
}
```

其中：

- `code`: 业务对应的 HTTP 状态码
- `message`: 文本消息
- `data`: 具体响应数据；失败时可能为 `null`

### 认证方式

支付相关接口需要登录。

推荐用于 agent / 服务端集成的方式：

```http
Authorization: Bearer <accessToken>
```

浏览器端也可以使用登录接口返回的 Cookie。

---

## 2. 推荐集成模式

### 模式 A：产品支付

适合场景：

- 平台内已有产品体系
- 用户购买会员、套餐、积分包
- 支付成功后需要自动履约

标准流程：

1. `POST /auth/login` 获取 `accessToken`
2. `POST /orders` 创建本地订单
3. `POST /payment/unified/create` 发起支付
4. 客户端或 agent 轮询 `GET /payment/unified/query`
5. 支付成功后后端自动把订单标记为 `paid`，并执行会员/积分履约

### 模式 B：直付支付

适合场景：

- 不想依赖产品表
- 需要让其他项目直接按金额发起支付
- 只需要支付单能力，不需要产品购买逻辑

标准流程：

1. `POST /auth/login` 获取 `accessToken`
2. `POST /payment/unified/direct/create` 创建直付单并拉起支付
3. 客户端或 agent 轮询 `GET /payment/unified/direct/query`
4. 支付成功后后端仅更新本地直付单状态，不自动走产品履约

如果是“其他项目通过 agent 调用”，优先建议使用“模式 B：直付支付”。

---

## 3. 登录接口

### POST /auth/login

用途：获取支付接口需要的 JWT。

请求体：

```json
{
  "email": "user@example.com",
  "password": "Test123456!",
  "projectId": "ytb2bili"
}
```

说明：

- `projectId` 可选
- 建议在多项目场景下显式传入，以便后端解析项目上下文

成功响应核心字段：

```json
{
  "code": 200,
  "message": "登录成功",
  "data": {
    "user": {
      "id": "13c9fe0c-ea23-4671-ac84-988d9ff788e1",
      "email": "user@example.com",
      "role": "user",
      "projectId": "ea8adc30a168b835d2fac68cee172433",
      "projectName": "ytb2bili",
      "projectSlug": "ytb2bili"
    },
    "accessToken": "<jwt>",
    "refreshToken": "<jwt>",
    "expiresIn": 604800
  }
}
```

常见错误：

- `401` 邮箱或密码错误
- `403` 账户被禁用，或邮箱未验证
- `404` 指定项目不存在
- `429` 登录频率过高

---

## 4. 产品支付接口

### POST /orders

用途：根据产品创建本地订单。

认证：需要 JWT / Cookie。

请求体：

```json
{
  "productId": "<product-id>"
}
```

成功响应：

```json
{
  "code": 201,
  "message": "订单创建成功",
  "data": {
    "id": "<order-id>",
    "orderNo": "ORD1744260000ABC123XYZ",
    "amount": 29.9,
    "productName": "月度会员",
    "status": "pending"
  }
}
```

### GET /orders

用途：获取当前登录用户的订单列表。

### GET /orders/{orderId}

用途：获取单个订单详情。

成功响应中的关键信息：

- `orderNo`
- `amount`
- `status`
- `paymentMethod`
- `paymentIntentId`
- `productName`

### POST /payment/unified/create

用途：基于已有订单发起统一支付。

请求体：

```json
{
  "orderId": "<order-id>",
  "paymentType": "wechat"
}
```

`paymentType` 支持：

- `wechat`
- `alipay`
- `paypal`

成功响应：

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "paymentType": "wechat",
    "orderNo": "ORD1744260000ABC123XYZ",
    "unifiedOrderNo": "UP202604100001",
    "data": {
      "qrCode": "https://...",
      "payUrl": "https://...",
      "paypalOrderId": null
    }
  }
}
```

### GET /payment/unified/query

用途：查询产品订单支付状态。

查询参数二选一：

- `orderId`
- `orderNo`

示例：

```text
GET /api/v1/payment/unified/query?orderId=<order-id>
```

成功响应：

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "localStatus": "paid",
    "unifiedStatus": 201,
    "unifiedStatusDesc": "支付成功",
    "paid": true,
    "order": {
      "orderNo": "ORD1744260000ABC123XYZ",
      "amount": 29.9,
      "status": "paid",
      "paidAt": 1744260123456
    }
  }
}
```

说明：

- 当上游状态为支付成功时，后端会自动将订单更新为 `paid`
- 同时会触发产品履约，例如会员激活或积分发放
- 重复查询是安全的，支付成功场景做了幂等处理

---

## 5. 直付支付接口

这组接口是当前最适合“其他项目或 agent 直接集成”的支付能力。

### POST /payment/unified/direct/create

用途：不依赖产品，直接创建直付单并拉起统一支付。

请求体：

```json
{
  "subject": "VIP 代充",
  "amount": 9.9,
  "description": "外部项目通过 agent 创建的支付单",
  "projectId": "ea8adc30a168b835d2fac68cee172433",
  "paymentType": "wechat"
}
```

字段说明：

- `subject`: 必填，支付标题，最长 120 字符
- `amount`: 必填，支付金额，必须大于 0
- `description`: 可选，支付描述，最长 500 字符
- `projectId`: 可选，用于绑定项目上下文
- `paymentType`: 可选，默认 `wechat`

成功响应：

```json
{
  "code": 200,
  "message": "直付支付单创建成功",
  "data": {
    "id": "<direct-payment-id>",
    "subject": "VIP 代充",
    "amount": 9.9,
    "currency": "CNY",
    "paymentType": "wechat",
    "orderNo": "PAY1744260000ABC123XYZ",
    "unifiedOrderNo": "UP202604100002",
    "data": {
      "qrCode": "https://...",
      "payUrl": "https://...",
      "paypalOrderId": null
    }
  }
}
```

常见错误：

- `400` 请求体不合法
- `401` 未登录
- `404` 传入的 `projectId` 不存在
- `502` 调用统一支付上游失败

### GET /payment/unified/direct/query

用途：查询直付单支付状态。

查询参数二选一：

- `directPaymentId`
- `orderNo`

示例：

```text
GET /api/v1/payment/unified/direct/query?directPaymentId=<direct-payment-id>
```

成功响应：

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "localStatus": "pending",
    "unifiedStatus": 1,
    "unifiedStatusDesc": "未支付",
    "paid": false,
    "payment": {
      "id": "<direct-payment-id>",
      "orderNo": "PAY1744260000ABC123XYZ",
      "subject": "VIP 代充",
      "amount": 9.9,
      "currency": "CNY",
      "status": "pending",
      "paidAt": null
    }
  }
}
```

### 状态说明

本地 `localStatus` / `payment.status` 可能值：

- `pending`
- `scanned`
- `paid`
- `failed`
- `cancelled`
- `refunded`

对应的统一支付状态码 `unifiedStatus`：

- `1`: 未支付
- `2`: 已扫码
- `101`: 支付失败
- `201`: 支付成功
- `300`: 已关闭
- `400`: 已退款

说明：

- 当前直付支付只负责记录支付单和同步支付状态
- 它不会自动触发产品履约
- 如果其他项目需要“支付成功后加积分 / 开会员 / 发货”，应在外部系统轮询到 `paid=true` 后执行自己的业务逻辑，或继续在本项目内扩展直付回调后的业务处理

---

## 6. Agent 集成建议

如果你在其他项目中让 agent 调用这些接口，推荐让 agent 遵循下面的约束。

### 推荐最小能力集

1. 登录并缓存 `accessToken`
2. 创建直付单
3. 返回 `qrCode` / `payUrl` 给上游调用方
4. 定时轮询查询接口
5. 当 `paid=true` 时结束流程

### 推荐 agent 工作流

```text
1. POST /api/v1/auth/login
2. POST /api/v1/payment/unified/direct/create
3. 提取 data.id、data.orderNo、data.data.qrCode、data.data.payUrl
4. 每隔 2~5 秒调用 GET /api/v1/payment/unified/direct/query
5. 当 paid=true 或 status 进入 failed/cancelled/refunded 时停止
```

### 推荐轮询策略

- 轮询间隔：`2` 到 `5` 秒
- 单次轮询时长：`1` 到 `3` 分钟
- 若超时仍未支付：返回“等待用户继续支付”，由上层稍后再次查询

### 不建议的做法

- 不要每次查询都重新登录
- 不要在网络超时时立即重复创建新的直付单
- 如果已经拿到 `directPaymentId` 或 `orderNo`，优先走查询接口，而不是重新创建支付单

---

## 7. cURL 示例

### 7.1 登录

```bash
BASE_URL="https://api2key-api-staging.<your-subdomain>.workers.dev/api/v1"

TOKEN=$(curl -sS -X POST "$BASE_URL/auth/login" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "Test123456!",
    "projectId": "ea8adc30a168b835d2fac68cee172433"
  }' | jq -r '.data.accessToken')
```

### 7.2 创建直付单

```bash
curl -sS -X POST "$BASE_URL/payment/unified/direct/create" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "subject": "VIP 代充",
    "amount": 9.9,
    "description": "外部 agent 测试",
    "projectId": "ea8adc30a168b835d2fac68cee172433",
    "paymentType": "wechat"
  }'
```

### 7.3 查询直付单状态

```bash
curl -sS "$BASE_URL/payment/unified/direct/query?directPaymentId=<direct-payment-id>" \
  -H "Authorization: Bearer $TOKEN"
```

### 7.4 产品支付流程

```bash
# 1. 创建订单
ORDER_JSON=$(curl -sS -X POST "$BASE_URL/orders" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"productId":"<product-id>"}')

ORDER_ID=$(echo "$ORDER_JSON" | jq -r '.data.id')

# 2. 发起支付
curl -sS -X POST "$BASE_URL/payment/unified/create" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"orderId\":\"$ORDER_ID\",\"paymentType\":\"wechat\"}"

# 3. 查询支付状态
curl -sS "$BASE_URL/payment/unified/query?orderId=$ORDER_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

## 8. Python 测试脚本

仓库内已有 Python 示例脚本，可直接用于联调：

- `examples/python/test_direct_payment.py`

示例：

```bash
python examples/python/test_direct_payment.py \
  --base-url "https://api2key-api-staging.<your-subdomain>.workers.dev/api/v1" \
  --email "user@example.com" \
  --password "Test123456!" \
  --project-id "ea8adc30a168b835d2fac68cee172433" \
  --amount 0.01 \
  --poll
```

---

## 9. 常见问题

### 创建直付单返回 500

先检查：

1. 目标环境是否已部署新代码
2. 目标环境的 D1 是否已执行 `scripts/migrate-add-direct-payments.sql`
3. `UNIFIED_PAYMENT_APP_SECRET` 等支付相关 secret 是否已配置在目标环境

### 创建直付单返回 502

这通常意味着本地接口已处理到调用统一支付上游，但上游创建支付失败。优先检查：

1. `UNIFIED_PAYMENT_API_URL`
2. `UNIFIED_PAYMENT_APP_ID`
3. `UNIFIED_PAYMENT_APP_SECRET`
4. 上游统一支付服务本身是否可用

### 为什么查询接口要支持 `id` 和 `orderNo`

- `id` 适合本系统内部追踪
- `orderNo` 适合跨系统传递与人工排查

---

## 10. 集成建议总结

如果是新项目接支付：

- 优先接 `POST /payment/unified/direct/create`
- 再接 `GET /payment/unified/direct/query`
- 使用 `POST /auth/login` 获取 JWT

如果是平台内部已有产品购买流：

- 继续使用 `POST /orders`
- 再接 `POST /payment/unified/create`
- 最后轮询 `GET /payment/unified/query`

对于 agent：

- 把“创建支付单”和“查询支付状态”当成两个独立动作
- 不要把“创建失败”和“未支付”混在一起处理
- 已经创建成功后，应优先查询，不应重复创建