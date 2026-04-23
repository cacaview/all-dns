# ALL-DNS

企业级跨云厂商 DNS 资产管理与运维平台。

## 技术栈

| 层       | 技术                                             |
| -------- | ------------------------------------------------ |
| 后端     | Go 1.22+ / Gin / GORM / PostgreSQL               |
| 前端     | Vue 3 / Vite / TypeScript / Element Plus / Pinia |
| 基础设施 | Docker / Docker Compose                          |

## 快速启动

```bash
# 1. 复制环境变量模板
cp .env.example .env
# 编辑 .env，填写必要的环境变量（见下方"环境变量"章节）

# 2. 启动完整服务（后台运行）
docker compose up -d --build

# 3. 查看服务状态
docker compose ps

# 4. 查看后端日志
docker compose logs -f server
```

启动后访问：

- 前端：http://localhost:5173
- 后端 API：http://localhost:8080

## 环境变量

必需的环境变量（`APP_MASTER_KEY` 和 `JWT_SECRET` 必须手动设置）：

| 变量                  | 说明                                         | 默认值                    |
| --------------------- | -------------------------------------------- | ------------------------- |
| `APP_MASTER_KEY`    | AES-256-GCM 主密钥，需 base64 编码的 32 字节 | **必须设置**        |
| `JWT_SECRET`        | JWT 签名密钥                                 | **必须设置**        |
| `DEV_LOGIN_ENABLED` | 启用开发登录端点（仅用于本地调试）           | `false`                 |
| `FRONTEND_URL`      | CORS 允许的前端源                            | `http://localhost:5173` |

对象存储（可选，用于业务档案附件）：

| 变量              | 说明                          | 默认值    |
| ----------------- | ----------------------------- | --------- |
| `STORAGE_TYPE`  | 存储类型：`local` 或 `s3` | `local` |
| `S3_ENDPOINT`   | S3 兼容端点                   | -         |
| `S3_BUCKET`     | S3 bucket 名称                | -         |
| `S3_ACCESS_KEY` | Access Key                    | -         |
| `S3_SECRET_KEY` | Secret Key                    | -         |
| `S3_REGION`     | Region                        | -         |

OAuth（可选，配置后启用 OAuth 登录）：

| 变量                                            | 说明                  |
| ----------------------------------------------- | --------------------- |
| `GITHUB_CLIENT_ID` / `GITHUB_CLIENT_SECRET` | GitHub OAuth 应用凭证 |
| `GITLAB_CLIENT_ID` / `GITLAB_CLIENT_SECRET` | GitLab OAuth 应用凭证 |
| `OAUTH_REDIRECT_URL`                          | OAuth 回调地址        |

传播检查解析器：

| 变量                      | 说明                          | 默认值                              |
| ------------------------- | ----------------------------- | ----------------------------------- |
| `PROPAGATION_RESOLVERS` | 公共 DNS 服务器列表，逗号分隔 | `1.1.1.1,8.8.8.8,114.114.114.114` |

## 功能概览

- **多云 DNS 适配**：Cloudflare、阿里云、腾讯云(DNSPod)、华为云、AWS Route53、GCP、DigitalOcean、Vultr、Hetzner、Namecheap、Mock
- **安全存储**：AES-256-GCM 加密云厂商凭证，JWT 鉴权
- **OAuth 登录**：支持 GitHub / GitLab
- **RBAC 权限**：admin / editor / viewer 三级角色
- **多租户隔离**：基于组织的账户和域名隔离
- **自动快照**：记录变更前全量快照，支持备份恢复
- **传播监控**：指定全球公共 DNS 轮询检查记录生效状态，支持持续 watch 模式
- **域名管理**：标星、标签、归档、业务档案（Markdown + 附件）
- **过期提醒**：支持 Webhook 通知和服务端持久化的已处理状态

## 目录结构

```
.
├── server/               # Go 后端
│   ├── cmd/api/          # 入口
│   ├── internal/
│   │   ├── config/       # 配置加载
│   │   ├── db/           # 数据库连接与迁移
│   │   ├── http/
│   │   │   ├── handler/  # HTTP 处理器
│   │   │   └── middleware/ # 中间件 (auth, RBAC)
│   │   ├── model/        # GORM 模型
│   │   ├── provider/    # DNSProvider 抽象与实现
│   │   ├── service/     # 业务逻辑
│   │   ├── storage/     # 存储抽象 (local/S3)
│   │   └── notifier/    # 提醒 Webhook
│   └── migrations/      # SQL 迁移文件
└── web/                 # Vue 3 前端
    └── src/
        ├── api/         # HTTP 客户端
        ├── stores/     # Pinia 状态管理
        ├── views/      # 页面组件
        └── components/ # 通用组件
```

## 开发

### 后端

```bash
cd server

# 运行测试
go test ./...

# 直接运行（需先启动 PostgreSQL）
go run ./cmd/api

# Docker 内开发（热重载）
docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build
```

### 前端

```bash
cd web

# 安装依赖
npm install

# 开发服务器
npm run dev

# 构建生产版本
npm run build
```
