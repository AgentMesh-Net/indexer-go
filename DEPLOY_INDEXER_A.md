# indexer-go v0.3.0 — 部署说明
## 节点：indexer-a.ainerwise.com

> **场景**：服务器已有 `registry`（registry.ainerwise.com）在运行，
> PostgreSQL / Nginx / Certbot 等中间件可共用，**只需新增 indexer-a 服务**。

---

## 目录

1. [前置条件](#1-前置条件)
2. [拉取代码 & 构建镜像](#2-拉取代码--构建镜像)
3. [数据库准备](#3-数据库准备)
4. [环境变量配置](#4-环境变量配置)
5. [Docker Compose（仅 indexer 服务）](#5-docker-compose仅-indexer-服务)
6. [Nginx 反代配置](#6-nginx-反代配置)
7. [启动 & 验证](#7-启动--验证)
8. [向 Registry 注册](#8-向-registry-注册)
9. [配置注意事项](#9-配置注意事项)
10. [版本升级流程](#10-版本升级流程)

---

## 1. 前置条件

| 依赖 | 要求 | 说明 |
|------|------|------|
| Docker | ≥ 24 | 构建 + 运行 |
| Docker Compose | v2+ | `docker compose` 命令 |
| PostgreSQL | 已有实例 | 共用 registry 的 PG，或独立 PG |
| Nginx | 已有 | 共用，新增 server block |
| SSL 证书 | Certbot 或已有通配符证书 | `*.ainerwise.com` 或单独申请 |
| Go（可选） | 1.24+ | 只在服务器上本地构建时需要 |

---

## 2. 拉取代码 & 构建镜像

```bash
# 进入你的项目目录
cd /opt/agentmesh   # 或你的实际部署目录

# 克隆（如果还没有）
git clone git@github.com:AgentMesh-Net/indexer-go.git
cd indexer-go

# 切到最新 tag
git fetch --tags
git checkout v0.3.0

# 构建 Docker 镜像（打上版本 tag）
docker build -t indexer-go:v0.3.0 -t indexer-go:latest .
```

---

## 3. 数据库准备

indexer 需要**独立的数据库**（不要和 registry 混用同一个 DB）。

### 3a. 如果共用已有 PostgreSQL 实例

```bash
# 连接 PG，创建 indexer-a 专属 DB 和用户
psql -U postgres

CREATE DATABASE indexer_a;
CREATE USER indexer_a WITH PASSWORD 'your_strong_password_here';
GRANT ALL PRIVILEGES ON DATABASE indexer_a TO indexer_a;
\q
```

### 3b. 如果使用独立 PostgreSQL 容器

见第 5 节 `docker-compose.yml` 示例，可选择性加入 postgres 服务。

### 迁移说明

**迁移由程序自动执行**，启动时按顺序运行：
- `001_init.sql` — 基础对象表（bids/accepts/artifacts）
- `002_tasks.sql` — 结构化任务表（tasks/accepts）
- `003_onchain_sync.sql` — 签名字段 + 链上同步字段

⚠️ **如果是从 v0.2.0 升级**，003 迁移会执行 `ALTER TABLE ADD COLUMN IF NOT EXISTS`，安全幂等，不会丢数据。

---

## 4. 环境变量配置

在服务器上创建 `/opt/agentmesh/indexer-go/.env.indexer-a`：

```bash
# ── 数据库 ────────────────────────────────────────────────────────
# 共用 PG 实例（修改为实际地址/密码）
AMN_DB_DSN=postgres://indexer_a:your_strong_password_here@localhost:5432/indexer_a?sslmode=disable

# ── HTTP 监听 ─────────────────────────────────────────────────────
# 监听本地端口，由 Nginx 反代
AMN_HTTP_ADDR=:8081

# ── Indexer 身份（必须填写）──────────────────────────────────────
INDEXER_NAME=indexer-a
INDEXER_BASE_URL=https://indexer-a.ainerwise.com
INDEXER_OWNER=ainerwise
INDEXER_CONTACT=ops@ainerwise.com
INDEXER_VERSION=0.3.0

# ── 手续费（bps，1 bps = 0.01%）─────────────────────────────────
# ≤100 bps → registry 自动 active；> 100 bps → pending 等待审核
INDEXER_FEE_BPS=20

# ── Ed25519 签名密钥（用于 /v1/meta 响应签名）────────────────────
# 生成方式（在本地执行）：
#   python3 -c "import secrets; print(secrets.token_hex(32))"
# 或：
#   openssl rand -hex 32
# ⚠️ 32 字节 hex（64 个字符），保管好，不要提交到 git
INDEXER_SIGNING_KEY=your_64_char_hex_ed25519_private_key_here

# ── 支持的链（JSON 数组）─────────────────────────────────────────
# chain_id: Sepolia=11155111, Mainnet=1
# settlement_contract: 部署的合约地址
# min_confirmations: 链上事件需要的确认块数
SUPPORTED_CHAINS_JSON=[{"chain_id":11155111,"settlement_contract":"0xf2223eA479736FA2c70fa0BB1430346D937C7C3C","min_confirmations":2}]

# ── 链上事件监听 RPC（可选，不填则 watcher 不启动）───────────────
# JSON map: chain_id(string) → rpc_url
# 支持 WebSocket（wss://）优先，HTTP（https://）降级轮询
# ⚠️ 建议用 Infura/Alchemy 的 WebSocket 端点以获得实时事件推送
# 如暂不需要链上同步，设为 {} 即可
INDEXER_RPC_URLS={"11155111":"wss://sepolia.infura.io/ws/v3/YOUR_INFURA_PROJECT_ID"}
```

### 生成 Ed25519 签名密钥

```bash
# 方式 1（推荐）
openssl rand -hex 32

# 方式 2
python3 -c "import secrets; print(secrets.token_hex(32))"
```

---

## 5. Docker Compose（仅 indexer 服务）

在 `/opt/agentmesh/indexer-go/` 创建 `docker-compose.prod.yml`
（**不包含 postgres 服务**，因为已有共用 PG）：

```yaml
# docker-compose.prod.yml
# 部署 indexer-a 节点（共用服务器上的 PG + Nginx）

services:
  indexer-a:
    image: indexer-go:v0.3.0
    container_name: indexer-a
    restart: unless-stopped
    env_file:
      - .env.indexer-a
    ports:
      - "127.0.0.1:8081:8081"   # 只暴露给本机 Nginx，不对外直接开放
    networks:
      - indexer-net
    healthcheck:
      test: ["CMD-SHELL", "wget -qO- http://localhost:8081/v1/health || exit 1"]
      interval: 15s
      timeout: 5s
      retries: 3
      start_period: 10s
    logging:
      driver: json-file
      options:
        max-size: "50m"
        max-file: "5"

networks:
  indexer-net:
    driver: bridge
```

> **如果 PG 也是 Docker 容器**（非宿主机直接安装），需要加 `extra_hosts`
> 或把 indexer-a 加入 registry 所在的 docker network，并修改 `AMN_DB_DSN` 用容器名。

---

## 6. Nginx 反代配置

在 `/etc/nginx/sites-available/indexer-a.ainerwise.com` 创建：

```nginx
server {
    listen 80;
    server_name indexer-a.ainerwise.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    server_name indexer-a.ainerwise.com;

    # SSL（如有通配符证书直接复用路径；否则用 certbot 单独申请）
    ssl_certificate     /etc/letsencrypt/live/ainerwise.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/ainerwise.com/privkey.pem;
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         HIGH:!aNULL:!MD5;

    # 安全头
    add_header X-Content-Type-Options nosniff;
    add_header X-Frame-Options DENY;
    add_header Strict-Transport-Security "max-age=31536000" always;

    # 请求大小限制（与 indexer MaxBodyBytes 2MB 保持一致）
    client_max_body_size 2m;

    location / {
        proxy_pass         http://127.0.0.1:8081;
        proxy_http_version 1.1;
        proxy_set_header   Host              $host;
        proxy_set_header   X-Real-IP         $remote_addr;
        proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
        proxy_read_timeout 30s;
        proxy_connect_timeout 5s;
    }
}
```

**申请证书（如需单独申请）**：
```bash
certbot --nginx -d indexer-a.ainerwise.com
```

**启用配置**：
```bash
ln -s /etc/nginx/sites-available/indexer-a.ainerwise.com \
      /etc/nginx/sites-enabled/

nginx -t && systemctl reload nginx
```

---

## 7. 启动 & 验证

```bash
cd /opt/agentmesh/indexer-go

# 启动
docker compose -f docker-compose.prod.yml up -d

# 查看启动日志
docker compose -f docker-compose.prod.yml logs -f indexer-a

# 预期日志内容：
# migration 001_init.sql applied
# migration 002_tasks.sql applied
# migration 003_onchain_sync.sql applied
# chain watcher started for chain=11155111 ...  （如果配置了 RPC URL）
# indexer listening on :8081
```

### 验证 API

```bash
# 健康检查
curl https://indexer-a.ainerwise.com/v1/health
# 期望：{"status":"up","latency_ms":...}

# Meta 信息（含 Ed25519 签名）
curl https://indexer-a.ainerwise.com/v1/meta
# 期望：{"name":"indexer-a","url":"https://indexer-a.ainerwise.com","fee_bps":20,"public_key":"...","signature":"...","chains":[...]}
```

---

## 8. 向 Registry 注册

indexer-a 启动后，需要在 registry 注册才能被 client 发现。

### 方式 A：调用 registry Admin API

```bash
curl -X POST https://registry.ainerwise.com/v1/admin/indexers \
  -H "Content-Type: application/json" \
  -H "X-Admin-Token: YOUR_REGISTRY_ADMIN_TOKEN" \
  -d '{
    "url": "https://indexer-a.ainerwise.com",
    "name": "indexer-a",
    "fee_bps": 20,
    "owner": "ainerwise",
    "contact": "ops@ainerwise.com"
  }'
```

> 注册后 registry 会自动触发一次健康探针，检查：
> - `GET /v1/health` → `status: "up"`
> - `GET /v1/meta` → `fee_bps` 与注册值一致
>
> 两项通过后状态变为 `active`。

---

## 9. 配置注意事项

### ⚠️ 关键配置项

| 环境变量 | 重要说明 |
|----------|---------|
| `INDEXER_SIGNING_KEY` | 32 字节 hex（64字符）Ed25519 私钥。**一旦注册到 registry 后不要更换**，registry 会缓存 `public_key` 做签名验证 |
| `INDEXER_FEE_BPS` | 必须与注册到 registry 时填写的值一致，否则健康探针会报 `fee_bps mismatch`，状态变为 `degraded` |
| `INDEXER_BASE_URL` | 必须填 `https://indexer-a.ainerwise.com`（与实际访问地址完全一致），registry 用此字段做 health probe |
| `AMN_DB_DSN` | 确保 PG 用户对 `indexer_a` 库有 DDL 权限（CREATE TABLE），首次启动需要建表 |
| `INDEXER_RPC_URLS` | 填 `{}` 则 watcher 不启动（纯 API 模式）；填 WebSocket 端点则实时监听链上事件 |
| `AMN_HTTP_ADDR` | 生产环境用 `:8081`（由 Nginx 反代），不要直接暴露 `:8080` 到公网 |

### 端口占用检查

```bash
# 确认 8081 没有被占用
ss -tlnp | grep 8081
```

### 多节点部署（indexer-b 等）

如果以后要部署 indexer-b：
- 复制 `.env.indexer-a` → `.env.indexer-b`
- 修改：`INDEXER_NAME=indexer-b`、`INDEXER_BASE_URL=https://indexer-b.ainerwise.com`、`AMN_DB_DSN`（用独立的 DB）、`AMN_HTTP_ADDR=:8082`
- 生成新的 `INDEXER_SIGNING_KEY`
- 在 `docker-compose.prod.yml` 加入 `indexer-b` 服务，端口映射 `127.0.0.1:8082:8082`
- 增加 Nginx server block

### RPC URL 安全

```bash
# ⚠️ INFURA PROJECT ID 是敏感信息，.env 文件不要提交 git
# .gitignore 已包含 .env*，确认：
grep ".env" /opt/agentmesh/indexer-go/.gitignore
```

### 数据库连接池

默认使用 pgx/v5 连接池，生产环境可在 `AMN_DB_DSN` 后追加参数：
```
postgres://...?pool_max_conns=20&pool_min_conns=2&sslmode=require
```

---

## 10. 版本升级流程

```bash
cd /opt/agentmesh/indexer-go

# 1. 拉取新版本
git fetch --tags
git checkout v0.X.0   # 替换为新版本号

# 2. 重新构建镜像
docker build -t indexer-go:v0.X.0 -t indexer-go:latest .

# 3. 更新 docker-compose.prod.yml 中的 image tag
sed -i 's/indexer-go:v0\.[0-9]\+\.[0-9]\+/indexer-go:v0.X.0/' docker-compose.prod.yml

# 4. 滚动重启（迁移自动执行）
docker compose -f docker-compose.prod.yml up -d --no-deps indexer-a

# 5. 验证
curl https://indexer-a.ainerwise.com/v1/health
curl https://indexer-a.ainerwise.com/v1/meta
```

---

## 快速参考

```
服务地址:  https://indexer-a.ainerwise.com
容器名:    indexer-a
内部端口:  8081
数据库:    indexer_a (独立 DB，共用 PG 实例)
镜像:      indexer-go:v0.3.0
配置文件:  /opt/agentmesh/indexer-go/.env.indexer-a
Compose:   /opt/agentmesh/indexer-go/docker-compose.prod.yml
Nginx:     /etc/nginx/sites-available/indexer-a.ainerwise.com
```

---

*indexer-go v0.3.0 — Phase 6: EIP-191 签名验证 + 链上事件监听*
