# TileDock

[中文](./README_CH.md) | [English](./README.md)

Docker Hub: <https://hub.docker.com/repository/docker/jiamum/tiledock/general>

一个基于 Go 1.22、Gin、GORM、SQLite 和 `html/template` 的轻量级自托管导航面板，主打拖拽和平铺布局。

当前版本采用：

- Go 服务端渲染
- 少量原生 JavaScript + `fetch`
- SQLite 单文件数据库
- 基于 Session 的登录认证
- `embed` 内嵌模板与静态资源，支持单文件分发
- Docker 部署支持

项目目标是先保证易维护、易部署、目录清晰，再逐步扩展。

## 功能概览

- 用户登录 / 退出登录
- 仪表盘首页
- 导航分组管理
- 导航链接增删改查
- 链接图标支持本地上传，未填写时可自动尝试抓取网站 favicon
- 链接与分组拖拽排序
- 系统设置浮动面板
- 本地上传仪表盘背景图，并自动压缩为 WebP 背景文件
- 上传新背景后会自动清理旧背景文件
- 背景虚化与黑色蒙板透明度调节
- 支持导入 Chrome / Firefox 书签 HTML 文件
- 关键管理操作审计日志
- `/healthz` 健康检查
- 统一错误处理
- 基础单元测试

## 技术栈

- Go 1.22+
- Gin
- GORM
- SQLite
- `html/template`
- 原生 JavaScript
- `log/slog`

## 项目结构

```text
.
├── cmd/server/main.go
├── config/config.yaml
├── internal
│   ├── config
│   ├── handler
│   ├── middleware
│   ├── model
│   ├── repository
│   ├── router
│   ├── service
│   └── view
├── pkg
│   ├── db
│   └── logger
├── static
│   ├── css
│   ├── js
│   └── uploads
├── templates
│   ├── auth
│   ├── dashboard
│   └── partials
├── Dockerfile
├── Makefile
└── README.md
```

## 首次初始化

当前版本不会再使用危险的默认管理员凭据。

首次启动时，如果 `users` 表为空，必须显式提供管理员账号和密码，否则程序会拒绝启动。

开发环境示例：

```bash
PANEL_DEFAULT_ADMIN_USER=admin \
PANEL_DEFAULT_ADMIN_PASSWORD='dev-password-123' \
make run
```

生产环境示例：

```bash
PANEL_APP_ENV=production \
PANEL_SESSION_SECRET='请替换成至少32位随机字符串' \
PANEL_SESSION_SECURE=true \
PANEL_DEFAULT_ADMIN_USER=admin \
PANEL_DEFAULT_ADMIN_PASSWORD='请替换成强密码' \
./bin/panel
```

## 配置方式

程序内置默认配置，但安全相关配置不会提供可用弱默认值。

如果存在 `config/config.yaml`，会在默认值基础上覆盖；环境变量优先级最高。

常用环境变量如下：

```bash
export PANEL_CONFIG="config/config.yaml"
export PANEL_SERVER_ADDR=":8080"
export PANEL_DB_PATH="data/panel.db"
export PANEL_UPLOAD_DIR="data/uploads"
export PANEL_BACKUP_DIR="data/backups"
export PANEL_SESSION_NAME="panel_session"
export PANEL_SESSION_SECRET="replace-with-at-least-32-random-characters"
export PANEL_SESSION_MAX_AGE="604800"
export PANEL_SESSION_SECURE="false"
export PANEL_SESSION_HTTP_ONLY="true"
export PANEL_DEFAULT_ADMIN_USER="admin"
export PANEL_DEFAULT_ADMIN_PASSWORD="strong-password"
export PANEL_LOG_LEVEL="info"
export PANEL_APP_ENV="development"
```

## 本地运行

要求：

- Go 1.22+
- CGO 可用

安装依赖并启动：

```bash
make tidy
PANEL_DEFAULT_ADMIN_USER=admin \
PANEL_DEFAULT_ADMIN_PASSWORD='dev-password-123' \
make run
```

或直接：

```bash
PANEL_DEFAULT_ADMIN_USER=admin \
PANEL_DEFAULT_ADMIN_PASSWORD='dev-password-123' \
go run ./cmd/server
```

启动后访问：

- 登录页：`http://localhost:8080/login`
- 仪表盘：`http://localhost:8080/`
- 健康检查：`http://localhost:8080/healthz`

## 单文件分发

当前版本已经支持单文件分发模式。

构建：

```bash
go build -o bin/tiledock ./cmd/server
```

分发时，最少只需要：

- `tiledock` 可执行文件

运行后程序会自动使用默认路径写入：

- `data/panel.db`
- `data/uploads`
- `data/backups`

可选项：

- 如果你要覆盖默认配置，再额外提供 `config/config.yaml`
- 如果你要自定义路径，也可以直接设置环境变量

## 常用命令

```bash
make run
make test
make build
make tidy
```

## Docker

构建镜像：

```bash
docker build -t panel:latest .
```

运行容器：

```bash
docker run --rm \
  -p 8080:8080 \
  -e PANEL_SESSION_SECRET="replace-with-at-least-32-random-characters" \
  -e PANEL_DEFAULT_ADMIN_USER="admin" \
  -e PANEL_DEFAULT_ADMIN_PASSWORD="strong-password" \
  -v "$(pwd)/data:/app/data" \
  panel:latest
```

说明：

- SQLite 数据库默认写入 `/app/data/panel.db`
- 上传的背景图片默认保存在 `/app/data/uploads`
- 备份文件默认保存在 `/app/data/backups`

## 当前页面与交互说明

- 仪表盘为主页面，左侧侧边栏已移除
- 系统设置通过顶部按钮打开浮动面板，不再作为独立管理页面使用
- 编辑模式下可以：
  - 新增分组
  - 新增链接
  - 编辑分组
  - 编辑链接
  - 删除分组和链接
  - 拖拽排序
- 非编辑模式下：
  - 点击链接会在新窗口打开
  - 编辑相关操作不会显示或生效

## 测试

运行全部测试：

```bash
go test ./...
```

## License

MIT，见 `LICENSE`。

## 部署注意事项

- 生产环境必须显式设置 `PANEL_SESSION_SECRET`，并至少 32 位
- 生产环境必须设置 `PANEL_SESSION_SECURE=true`
- 首次启动必须显式提供 `PANEL_DEFAULT_ADMIN_USER` 和 `PANEL_DEFAULT_ADMIN_PASSWORD`
- 建议持久化挂载 `data`
- 如果放在反向代理后面，仍应使用 HTTPS
- 备份恢复现在要求再次输入当前密码，并限制上传 zip 大小
- 系统设置支持导入 Chrome / Firefox 导出的书签 HTML，自动转换为分组和链接
- 背景图片只接受本地上传生成的 `/static/uploads/backgrounds/...` 路径
- 关键操作会写入结构化审计日志，例如登录、设置修改、导航 CRUD、备份导出与恢复
