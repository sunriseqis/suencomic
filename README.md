# SUENCOMIC // 包豪斯多源漫画下载平台

> 基于 **Go + Vue 3** 架构的多漫画源智能抓取与格式转换下载系统。  
> 采用经典 **包豪斯（Bauhaus）风格** 视觉设计，追求极简、功能主义与几何美学。

---

## 🎨 核心特性

- 🏠 **全新首页（HOME // 热门专区）**
  - **全网多源实时热度榜**：实时抓取 CopyManga / DM5 / MangaBZ 三大源站热门榜单数据，带 DNS 污染防护（直连结果校验失败自动切换代理重试）。
  - **防毒直连校验**：所有源站请求默认校验 TLS 证书，被污染的直连响应（返回 200 的垃圾页）会被识别并走代理重试。
  - **全网搜索与智能相关性排序**：三源聚合搜索，简繁同形归一化匹配。
- 🌐 **三大主流漫画源整合**
  - **CopyManga (拷贝漫画)**: 原生 API 协议解析与 AES-128-CBC 解密。
  - **DM5 (动漫屋)**: 章节目录解析与 Dean Edwards JS 解密流。
  - **MangaBZ (漫画BZ)**: 搜索、章节元数据与 Dean Edwards 高清图片反混淆。
- ⚡ **自动测速与智能故障换源 (Smart Auto-Fallback)**
  - 实时并发探测三大源响应延迟（ms）与可用性，标记当前最快源。
  - 下载过程中若遇到单话缺失、HTTP 429 限流、403 防盗链或网络超时，**自动无缝切换备用源**并对齐对应话数，保障下载成功率。
- 📦 **多格式打包转换**
  - **PDF (单本高清)**: 高保真自适应图片尺寸合并为单本 PDF 漫画。
  - **RAW (原始图片)**: 保持原始分辨率图片序列 (`0001.jpg`, `0002.jpg` 等)。
  - **CBZ (漫画压缩包)**: 封装包含 `ComicInfo.xml` 漫画元数据的标准 CBZ 档案。
  - **EPUB (电子书格式)**: 包含标准 OPF/NCX 目录与分页 XHTML 的 EPUB3 电子书。
- 📁 **固定输出路径**
  - 默认且固定输出目录为 `./download`，支持多层级按漫画名称与章节归类存储。
  - 支持断点续传（下载异常退出后重启自动复用已有图片缓存）。
- 📡 **实时 SSE 任务队列监控**
  - 实时推送下载进度、已下载图片数、打包耗时及换源溯源日志。
- 🔔 **追更订阅与定时自动下载**
  - 收藏订阅漫画，后台按设定周期检测最新章节并自动推送下载。
- 🛡️ **代理与网络支持**
  - 支持在 Web 界面或配置文件中配置全局 `HTTP` / `HTTPS` / `SOCKS5` 代理（如 `socks5://127.0.0.1:7890`）。

---

## 🏗️ 架构设计

```
/home/suencom/
├── internal/
│   ├── config/               # 配置管理 (config.json)
│   ├── sources/              # 漫画源解析器 (CopyManga / DM5 / MangaBZ / Unpacker)
│   ├── downloader/           # 下载任务队列、断点续传与格式打包 (PDF/CBZ/EPUB)
│   ├── tracker/              # 追更订阅检测调度器 (subscriptions.json)
│   └── api/                  # RESTful API & SSE 事件流服务
├── web/                      # Bauhaus 设计风格前端 (Vue 3 + Vite)
│   └── dist/                 # 编译打包生成的前端静态资源
├── download/                 # 固定的漫画下载输出目录
├── Dockerfile                # 多阶段轻量化 Docker 构建
├── docker-compose.yml        # Docker Compose 服务编排
├── build.sh                  # 一键编译打包脚本
├── Makefile                  # 标准自动化构建工具
└── main.go                   # Go 服务主入口 (通过 embed.FS 打包全量静态资产)
```

---

## 🚀 部署与运行

### 方式 1: 单二进制编译部署 (Compiled Deployment)

项目支持将前后端全量打包为单一可执行文件，开箱即用，无外部依赖：

```bash
# 1. 运行一键构建脚本（或使用 make build）
./build.sh

# 2. 启动服务（支持自定义端口，默认 8090）
./suencomic -port 8090
```

启动后在浏览器中访问：`http://localhost:8090`。

---

### 方式 2: Docker / Docker Compose 部署 (Docker Deployment)

使用 Docker 容器化部署，自动挂载 `./download` 下载目录与配置文件：

```bash
# 首次部署前先创建状态文件（避免 Docker 将单文件挂载误创建为目录）：
touch tasks.json subscriptions.json

# 启动 Docker 服务
docker compose up -d --build

# 查看运行日志
docker compose logs -f
```

> ⚠️ 更新前端代码后务必加 `--build` 重新构建镜像；若升级后浏览器仍显示旧界面（旧源/旧 Tab），是 index.html 被浏览器缓存所致，强制刷新一次（Ctrl+F5）即可。

---

## ⚙️ 配置文件说明 (`config.json`)

系统在首次启动时会自动生成 `config.json`，亦可在前端「CONFIG / 系统配置」页面中可视化修改：

```json
{
  "download_dir": "./download",
  "proxy": "socks5://127.0.0.1:7890",
  "max_concurrent_chapters": 3,
  "max_concurrent_images": 5,
  "auto_fallback": true,
  "check_interval_minutes": 60,
  "default_format": "pdf",
  "port": 8090,
  "skip_tls_verify": false
}
```

- `proxy`：留空时回退读取 `HTTPS_PROXY` / `HTTP_PROXY` / `ALL_PROXY` 环境变量（便于 Docker 注入）。
- `check_interval_minutes`：追更订阅检测间隔（分钟），改动即时生效，无需重启。
- `skip_tls_verify`：默认 `false`（校验源站证书）。仅当你的网络环境存在 TLS 中间人（透明代理）导致源站全部报证书错误时才设为 `true`。

---

## 📐 Bauhaus (包豪斯) 视觉风格特性

前端严格遵循包豪斯艺术流派设计语言：
1. **纯粹原色搭配**：包豪斯红 (`#D02C24`)、钴蓝 (`#194A8D`)、镉黄 (`#FDB813`) 与黑白对比。
2. **几何结构美学**：圆形、矩形、三角形标志，硬朗的 2.5px 黑色结构线与无羽化硬阴影 (`4px 4px 0px #111`)。
3. **功能至上布局**：非对称网格系统、清晰的序号索引 (`// 01 EXPLORE`) 与动态斑马线加载条。
