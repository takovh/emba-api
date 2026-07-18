# EMBA API

将 [EMBA](https://github.com/e-m-b-a/emba)（嵌入式固件安全分析器）封装为 RESTful API 服务。

- 远程提交固件扫描任务
- 实时查看扫描进度（SSE 事件流）
- 获取扫描结果（日志、HTML 报告、SBOM 清单）

## 技术栈

- **语言**: Go 1.22+
- **路由**: 标准库 `net/http`（Go 1.22 ServeMux）
- **数据库**: SQLite（通过 modernc.org/sqlite，纯 Go 实现）
- **前端**: 单页 HTML（内嵌于二进制中）

## 快速开始

### 从源码构建

```bash
# 安装 Go 1.22+
# https://go.dev/dl/

git clone <this-repo>
cd emba-api
go build -o emba-api .
```

### 运行

```bash
# 直接运行
./emba-api

# 或指定自定义配置
EMBA_HOME=/path/to/emba EMBA_LOG_DIR=/data/emba-logs ./emba-api
```

### 安装为 systemd 服务

```bash
sudo ./scripts/install.sh
```

### 卸载

```bash
sudo ./scripts/uninstall.sh
```

## 配置

所有配置通过环境变量设置：

| 环境变量 | 默认值 | 说明 |
|---------|--------|------|
| `EMBA_HOST` | `0.0.0.0` | 监听地址 |
| `EMBA_PORT` | `8203` | 监听端口 |
| `EMBA_HOME` | `/home/tako/workspace/emba` | EMBA 安装目录 |
| `EMBA_LOG_DIR` | `/home/tako/workspace/emba-log` | 扫描日志根目录 |
| `EMBA_MAX_CONCURRENT_SCANS` | `1` | 最大并发扫描数 |

## API 文档

### `GET /api/version`

查询 EMBA 版本。

```json
{"version": "1.4.2", "emba_home": "/home/tako/workspace/emba"}
```

### `GET /api/scan/profiles`

列出可用扫描配置文件。

```json
["default.emba", "quick.emba"]
```

### `POST /api/scan`

创建新扫描任务。请求格式为 `multipart/form-data`。

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `firmware` | file | 是 | 固件文件 |
| `modules` | string | 否 | 模块选择（逗号分隔，如 `S10,S25`） |
| `profile` | string | 否 | 扫描配置文件名（如 `default.emba`） |
| `arch` | string | 否 | 指定架构 |
| `name` | string | 否 | 自定义任务名称 |

```json
{"task_id": "a1b2c3d4e5f6", "status": "running", "message": "Scan task created"}
```

### `GET /api/scan`

列出所有任务（分页）。

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `page` | int | 1 | 页码 |
| `page_size` | int | 20 | 每页数量 |

```json
{
  "total": 42,
  "page": 1,
  "page_size": 20,
  "items": [
    {
      "task_id": "a1b2c3d4e5f6",
      "name": "firmware.bin",
      "status": "running",
      "elapsed_seconds": 123.4,
      "created_at": "2026-07-14T10:00:00Z",
      "completed_at": null
    }
  ]
}
```

### `GET /api/scan/{task_id}`

查询单个任务状态。

```json
{
  "task_id": "a1b2c3d4e5f6",
  "name": "firmware.bin",
  "status": "completed",
  "elapsed_seconds": 3600.0,
  "created_at": "2026-07-14T10:00:00Z",
  "completed_at": "2026-07-14T11:00:00Z",
  "exit_code": 0
}
```

### `GET /api/scan/{task_id}/logs`

获取完整扫描日志（纯文本）。

### `GET /api/scan/{task_id}/report`

下载 HTML 报告（ZIP 压缩包）。

### `GET /api/scan/{task_id}/sbom`

下载 SBOM JSON（CycloneDX 格式）。

### `DELETE /api/scan/{task_id}`

删除任务及所有相关文件。

```json
{"message": "Task deleted"}
```

### `GET /api/scan/{task_id}/events`

SSE（Server-Sent Events）实时进度流。

支持 `Last-Event-ID` 请求头实现断线重连。

```
id: 1
data: {"type":"progress","message":"Scan started","timestamp":"2026-07-14T10:00:00Z"}

id: 2
data: {"type":"log","message":"[EMBA] Starting firmware analysis...","timestamp":"2026-07-14T10:00:02Z"}

id: 3
data: {"type":"completed","message":"Scan finished (exit=0)","timestamp":"2026-07-14T11:00:00Z"}
```

## 任务生命周期

```
pending → running → completed (exit_code == 0)
                  → failed    (exit_code != 0)
                  → (通过 DELETE API 直接删除)
```

## 许可证

MIT License

Copyright (c) 2026 takovh
