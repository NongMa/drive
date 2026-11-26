# NongMa Drive

基于 [MyGo](https://mygo.run/zh) 原生 UI 的网盘桌面客户端。界面由 Go 绘制（Liquid Glass 材质、无 WebView），本身不含服务端：登录、文件、上传、分享等接口全部由另一个项目 [NongMa/resource-hub](https://github.com/NongMa/resource-hub) 提供，需要你自行部署该服务并填入自己的访问地址。

## 运行

需要 Go 1.27+。MyGo CLI 通过 `go.mod` 的 `tool` 指令固定版本，无需安装额外工具。

```powershell
go tool mygo dev
```

先部署服务端 [resource-hub](https://github.com/NongMa/resource-hub)（Next.js + Supabase/PostgreSQL + 七牛云），拿到它的访问地址后写入本地 `.env`（已 gitignore，模板见 `.env.example`）：

```env
RESOURCE_HUB_API=https://your-own-deployment.example.com/
```

真实环境变量优先于文件；没有配置时兜底为 `http://localhost:3000`（本地起服务端）。登录后会话 cookie 保存在 `os.UserConfigDir()/resource-hub-desktop/session.json`，重启自动恢复；下载目录和并发数保存在同目录的 `settings.json`。

## 功能

- 登录：图形验证码 + 服务端票据 + NextAuth 会话
- 文件：列表/网格双视图、网盘式混排表格（文件名 / 大小 / 类型 / 修改时间）、搜索、按上传时间正序/倒序、应用内预览、应用内下载（圆形进度）、分享、删除
- 文件夹：可展开目录树、文件夹卡片、面包屑、新建文件夹弹窗
- 上传：类型筛选、多选、最多 10 并发（可配置）、上传中心逐项进度
- 批量下载：并发数可设置（默认 1，最大 10），超出排队
- 界面：Liquid Glass 玻璃材质、3D 倾斜悬浮卡片、圆角图标按钮

## 打包与发布

本地打包：

```powershell
go tool mygo build
```

产物在 `build/<platform>/`。CI 见 `.github/workflows/release.yml`：推送 `v*` 标签时，在 macOS / Windows 运行器上分别构建并创建（或更新）同名 GitHub Release，上传 `.dmg` / `.exe` 安装包。原生 UI 项目不需要 Bun/前端构建。

未配置代码签名时，macOS 为临时签名、Windows 未签名；正式分发需配置 Apple Developer ID / 公证凭据和 Windows 代码签名证书。

- 登录链路：`/api/auth/captcha` → `/api/auth/login` → `/api/auth/csrf` → `/api/auth/callback/credentials`
- 界面代码：`main.go` / `icons.go`；API 客户端：`api/client.go`
