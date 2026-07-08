# EMBA RESTful API 设计文档

## 概述

将 EMBA 固件安全分析器封装为 RESTful API 服务，支持远程提交固件扫描任务、查询进度和获取结果。

## 技术栈

| 组件 | 选型 | 理由 |
|---|---|---|
| API 框架 | FastAPI (Python) | 原生 async、自动 OpenAPI 文档、Pydantic 校验 |
| 任务执行 | subprocess 调用 ./emba | 简单直接，复用 EMBA 现有能力 |
| 异步任务 | FastAPI BackgroundTasks | 处理耗时的扫描任务 |
| 实时进度 | SSE (Server-Sent Events) | 服务端推送，自动重连，实现简单 |
| 部署方式 | 裸机部署 | 直接在宿主机运行 |

## 项目结构

```
emba-api/
├── pyproject.toml       # 项目配置 + 依赖声明
├── uv.lock              # 依赖锁定文件
├── main.py              # FastAPI 应用入口
├── config.py            # 配置项（EMBA 路径、日志目录等）
├── database.py          # SQLite 数据库操作
├── models.py            # Pydantic 数据模型
├── tasks.py             # 后台任务管理（启动 EMBA、监控进程）
├── routers/
│   └── scan.py          # /api/scan 路由
├── utils/
│   └── log_parser.py    # 解析 EMBA 日志，提取进度信息
└── static/
    └── index.html       # 单页前端（HTML + CSS + JS）
```

## API 端点

### 1. 查询 EMBA 版本

**GET /api/version**

响应 200：

```json
{
  "version": "2.0.0",
  "emba_path": "/home/gst/yzhang/emba"
}
```

### 2. 启动扫描

**POST /api/scan**

请求（multipart/form-data）：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| firmware | File | 是 | 固件文件 |
| modules | string | 否 | 模块选择，如 "S10,S25" |
| profile | string | 否 | 扫描配置文件名 |
| arch | string | 否 | 指定架构 |

响应 201：

```json
{
  "task_id": "abc123",
  "status": "pending",
  "message": "Scan task created"
}
```

### 3. 查询任务状态

**GET /api/scan/{task_id}**

响应 200：

```json
{
  "task_id": "abc123",
  "status": "running",
  "elapsed_seconds": 120,
  "created_at": "2026-07-07T10:00:00Z",
  "completed_at": null,
  "exit_code": null
}
```

status 可选值：`pending` | `running` | `completed` | `failed`

### 4. 获取完整日志

**GET /api/scan/{task_id}/logs**

响应 200：返回 `emba.log` 文本内容

### 5. 下载 HTML 报告

**GET /api/scan/{task_id}/report**

响应 200：返回 `html-report/` 目录的 zip 压缩包

### 6. 下载 SBOM

**GET /api/scan/{task_id}/sbom**

响应 200：返回 `EMBA_cyclonedx_sbom.json`

### 7. 删除任务

**DELETE /api/scan/{task_id}**

响应 200：删除任务及所有相关文件

### 8. SSE 实时进度

**GET /api/scan/{task_id}/events**

连接后持续接收 SSE 事件，每个事件格式：

```
id: 1
data: {"type":"progress","message":"...","timestamp":"2026-07-07T10:02:00Z"}
```

- 支持 `Last-Event-ID` 头实现断线重连
- 新客户端（无 `Last-Event-ID`）会收到缓冲区全部历史消息
- 重连客户端只收到 `Last-Event-ID` 之后的消息

type 可选值：`progress` | `log` | `completed` | `error`

## 核心实现设计

### 任务管理

- SQLite 数据库存储任务状态（`database.py`）
- 运行时信息存储在内存字典 `_runtime`
- 任务完成后自动清理固件临时文件，保留结果日志
- 支持手动删除任务及所有相关文件

### EMBA 调用方式

通过 `subprocess.Popen` 启动，使用绝对路径 + `sudo`，并切换到 EMBA 目录执行：

```python
subprocess.Popen(
    ["sudo", EMBA_PATH + "/emba", "-f", fw_path, "-l", log_dir, "-p", profile, ...],
    cwd=EMBA_PATH,
    stdout=subprocess.PIPE,
    stderr=subprocess.STDOUT
)
```

> - `EMBA_PATH` 来自 `config.py`，默认值 `/home/gst/yzhang/emba`
> - `cwd=EMBA_PATH` 确保在 EMBA 项目目录下执行，模块和配置能正确加载
> - `-p <profile>` 指定扫描配置文件，文件位于 `${EMBA_PATH}/scan-profiles/<profile>.emba`

### 进度检测

后台线程每 2 秒读取 `${LOG_DIR}/emba.log`，通过以下方式判断进度：

- 匹配 `"preparing modules"` / `"running module"` 等关键词
- 解析当前执行的模块名（如 `S09_firmware_base_version_check`）
- 扫描完成标志：日志中出现 `"Test phase ended"` 或进程退出

### 结果路径映射

| 资源 | 路径 |
|---|---|
| 主日志 | `${LOG_DIR}/emba.log` |
| 控制台输出 | `${LOG_DIR}/emba.console.log` |
| HTML 报告 | `${LOG_DIR}/html-report/` |
| SBOM | `${LOG_DIR}/SBOM/EMBA_cyclonedx_sbom.json` |
| CVE 分析 | `${LOG_DIR}/f17_cve_bin_tool/` |
| CSV 数据 | `${LOG_DIR}/csv_logs/` |

### 并发控制

- 配置 `EMBA_MAX_CONCURRENT_SCANS` 限制同时运行的扫描数（默认 1）
- 超出时返回 HTTP 503 Service Unavailable

## 配置项 (config.py)

| 配置 | 默认值 | 说明 |
|---|---|---|
| EMBA_PATH | `/home/gst/emba` | EMBA 项目根目录 |
| EMBA_LOG_BASE_DIR | `/home/gst/emba-log` | 扫描日志基础目录 |
| EMBA_MAX_CONCURRENT_SCANS | 1 | 最大并发扫描数 |
| EMBA_HOST | `0.0.0.0` | 监听地址 |
| EMBA_PORT | 8203 | 监听端口 |

## 部署方式

使用 [uv](https://docs.astral.sh/uv/) 管理依赖和虚拟环境：

```bash
cd emba-api
uv sync              # 安装依赖
uv run main.py       # 启动服务
```

## 前端页面

提供一个简单的单页 HTML 页面，通过浏览器操作 API。页面由 FastAPI 静态文件托管，访问 `http://localhost:8000/` 即可打开。

### 功能

- 查询 EMBA 版本
- 上传固件文件并启动扫描（选择配置文件）
- 实时显示扫描进度（SSE）
- 查看/下载日志、报告、SBOM
- 任务列表管理

### 页面布局

```
┌─────────────────────────────────────────────────┐
│  EMBA Scanner                          v2.0.0    │
├─────────────────────────────────────────────────┤
│  [上传固件]                                      │
│  文件: [________] 配置: [________] [开始扫描]     │
├─────────────────────────────────────────────────┤
│  任务列表                                        │
│  ┌─────┬──────────┬────────┬──────────────────┐ │
│  │ ID  │ 状态     │ 耗时   │ 操作             │ │
│  ├─────┼──────────┼────────┼──────────────────┤ │
│  │ abc │ running  │ 2m30s  │ 日志 报告 SBOM ✕ │ │
│  └─────┴──────────┴────────┴──────────────────┘ │
├─────────────────────────────────────────────────┤
│  [实时日志输出区域]                               │
└─────────────────────────────────────────────────┘
```

### 技术方案

| 组件 | 选型 | 说明 |
|---|---|---|
| 页面 | 单个 `index.html` | 内嵌 CSS + JS，无构建工具 |
| 样式 | 原生 CSS | 简洁暗色主题 |
| HTTP 请求 | 原生 `fetch` | 无需 jQuery/axios |
| 实时更新 | 原生 `EventSource` API | SSE 自动重连 |
| 文件托管 | FastAPI `StaticFiles` | 挂载 `static/` 目录 |

### 文件结构

```
emba-api/
├── static/
│   └── index.html       # 单页前端（HTML + CSS + JS）
```

### 页面交互流程

1. **页面加载** → 调用 `GET /api/version` 显示版本号
2. **选择文件** → 用户选择固件文件和配置文件
3. **点击扫描** → `POST /api/scan` 上传文件，返回 task_id
4. **建立 SSE** → 连接 `GET /api/scan/{task_id}/events` 接收进度
5. **进度更新** → 实时更新 SSE 推送的消息
6. **扫描完成** → 自动刷新任务列表，显示结果链接
7. **下载结果** → 点击按钮调用对应 API 下载日志/报告/SBOM
