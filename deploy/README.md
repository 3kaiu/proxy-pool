# 代理池部署方案

## 重要说明

**opencode.ai 部署在 Cloudflare 上**，因此 Cloudflare Worker 直连 opencode.ai 会被同站过滤（CF 识别到请求来自自家 Worker，会拦截或返回错误）。CF Worker 方案仅适用于目标站不在 Cloudflare 上的场景。

代理池方案通过公开代理 IP 轮换绕过 IP 限制，适用于任何目标站。

---

## 方案一：Koyeb（推荐 - 免费、不要信用卡）

**原理**：把完整代理池服务部署到 Koyeb 免费 Docker 容器，通过 relay 端点用代理池轮换 IP 访问 opencode.ai。

**优点**：
- 免费层：512MB RAM / 0.1 vCPU / 2GB SSD
- **不需要信用卡**（GitHub/Google 登录即可）
- 不限制出站端口（可以连 HTTP/SOCKS 代理）
- 支持 Docker 一键部署
- 自动 HTTPS

**缺点**：
- 闲置会休眠（用 UptimeRobot 每 5 分钟 ping 防休眠）
- 0.1 vCPU 性能有限（调低 `MaxValidateConcurrency`）
- 2026.02 被 Mistral AI 收购，免费层政策可能变化

**部署步骤**：

1. 注册 [koyeb.com](https://www.koyeb.com)（GitHub 登录）
2. Fork 本仓库到你的 GitHub
3. Koyeb Dashboard → Create Service → GitHub → 选择仓库
4. 构建类型选 Docker，端口设 5010
5. 环境变量：
   ```
   PROXY_POOL_LISTEN=:5010
   PROXY_POOL_DB_PATH=/app/data/proxies.db
   PROXY_POOL_TARGET_URL=https://opencode.ai/zen/v1/models
   ```
6. 部署，拿到 `https://your-app.koyeb.app` 地址
7. 防休眠：[UptimeRobot](https://uptimerobot.com)（免费）每 5 分钟 ping `https://your-app.koyeb.app/count`

**使用**：
```bash
# 模型列表
curl https://your-app.koyeb.app/v1/models

# 聊天（deepseek-v4-flash-free 是 reasoning 模型，max_tokens 需设 2000+）
curl https://your-app.koyeb.app/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-v4-flash-free",
    "messages": [{"role": "user", "content": "Hello"}],
    "max_tokens": 2000
  }'

# 代理池状态
curl https://your-app.koyeb.app/count
```

**OpenAI 兼容客户端**：直接把 `base_url` 设为 `https://your-app.koyeb.app/v1`

---

## 方案二：Render（备选 - 免费但会休眠）

**原理**：同 Koyeb，部署完整代理池到 Render 免费 Web Service。

**优点**：
- 不需要信用卡
- 支持 Docker

**缺点**：
- 15 分钟无请求自动休眠（首次请求冷启动慢）
- 文件系统不持久（SQLite 重启丢失，代理池需重新抓取）
- 免费层 512MB RAM

**部署**：
1. 注册 [render.com](https://render.com)
2. New → Web Service → 连接 GitHub 仓库
3. 环境选 Docker，端口 5010
4. 环境变量同 Koyeb

---

## 方案三：Cloudflare Worker（仅适用于非 CF 站点）

> ⚠️ **对 opencode.ai 不可用**：opencode.ai 在 Cloudflare 上，CF Worker 直连会被同站过滤。

如果你的目标 API 不在 Cloudflare 上，CF Worker 是最简单的方案——利用 CF 全球边缘节点的天然 IP 轮换。

**部署**：
```bash
cd deploy/cf-worker
npx wrangler deploy
```

---

## 方案四：Fly.io（需信用卡）

> ⚠️ Fly.io 现在要求绑定信用卡，不再有真正的免费层。

如果你有信用卡，Fly.io 提供 3 个免费 shared-cpu VM + 3GB 存储。

**部署**：
```bash
flyctl launch --no-deploy
flyctl volumes create proxy_data --region nrt
flyctl deploy
```

---

## 各平台对比

| 平台 | 免费 | 不要信用卡 | 持久存储 | TCP代理 | 休眠 | 适合 opencode.ai |
|------|------|-----------|---------|---------|------|-----------------|
| **Koyeb** | ✅ | ✅ | ✅ | ✅ | 可防 | ✅ |
| Render | ✅ | ✅ | ❌ | ✅ | 15min | ✅ |
| CF Worker | ✅ | ✅ | ❌ | ❌ | 不休眠 | ❌ 同站过滤 |
| Fly.io | ❌ | ❌ | ✅ | ✅ | 不休眠 | ✅ |
| Oracle Cloud | ✅ | ❌ | ✅ | ✅ | 不休眠 | ✅ |

## 推荐

- **访问 opencode.ai/zen** → **Koyeb**（方案一）：免费、不要信用卡、完整代理池
- **备选** → Render（方案二）：免费但会休眠、存储不持久
- **其他非 CF 站点** → CF Worker（方案三）：最简单，30 秒部署
