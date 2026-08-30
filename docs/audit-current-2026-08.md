# 当前工作树全面审计

日期：2026-08-23

范围：`redesign/v1.0` 当前提交（含本轮 GUI redesign），覆盖 Go 引擎、Wails/CLI、前端、脚本、CI、依赖、文档与测试。

## 结论

当前工作树已通过本地可执行的 Go、前端、静态分析和密钥扫描；此前失效的浏览器测试已同步到新 UI，并扩展为 16 个场景。未执行真实 SSH/Tailscale/防火墙 Apply，因此本审计不等同于真实主机或 Windows 发布验收。Windows 原生 Pester/Wails/NSIS/升级测试仍必须由 CI runner 完成。

## 本轮修复

- 重写过时的 Playwright 选择器与断言，覆盖首次安装、无公钥、清空公钥、LAN 切换、语言持久化、配置导入导出、装机卡、XSS 文本渲染、离线 Tailscale blocker、repair、防止假健康、UAC 取消、幂等回访、窄屏和键盘无障碍。
- 修复 GUI 健康判断：公钥匹配、额外防火墙范围、非精确端口规则、未知防火墙 provider 和 `exposure: none` 均不再被误报为 ready；plan blocker 仍保留“更改网络/钥匙”的修复入口。
- 修复浏览器 YAML 导出丢失设置、潜在 auth key 脱敏不足、硬编码高级选项、progressbar 语义和 build 后 `dist/.gitkeep` 被删除的问题。
- Unix `authorized_keys` Apply/rollback 增加 symlink、非普通文件、backup 冲突和临时文件保护；防火墙多 CIDR 命令失败时执行 action 内回滚。
- 防火墙探测不再把覆盖多个端口的规则当作精确规则；Windows、UFW 和 firewalld 均记录端口范围并 fail closed。
- 延迟 self-cut fallback 记录 PID，rollback 可取消无 systemd 的后台动作；CLI 防火墙动作补齐端口参数。
- 下载/镜像 URL 拒绝嵌入凭据，失败信息去除 URL 凭据；代理能力明确限制为实际实现的 HTTP/HTTPS（不再虚假接受 SOCKS5）。
- 修复前端依赖审计发现的 PostCSS/nanoid 漏洞（pnpm override + lockfile）；加入 `.pi/` 忽略规则；同步 README、开发文档和 release metadata 测试。

## 验证证据

通过：

- `go test ./... -count=1`
- `go test -race ./...`
- `go vet ./...`
- `go build -trimpath ./...`
- CLI 交叉编译：Windows amd64/arm64、Linux arm64、macOS amd64/arm64
- `staticcheck ./...`（v0.7.0）
- `gosec -exclude-generated -severity high ./...`：0 issues
- `GOTOOLCHAIN=go1.25.13 govulncheck ./...`：无可达漏洞
- ShellCheck：bootstrap、offline-pack、macOS launcher
- gitleaks：42 commits，no leaks
- `cd frontend && pnpm install --frozen-lockfile`
- `pnpm run typecheck && pnpm run build`
- `pnpm run test:e2e`：16 passed
- `pnpm audit --audit-level high`：No known vulnerabilities
- Windows PowerShell/Pester：release metadata、bootstrap、package smoke 等 25 项通过；离线包 2 项仅因从 Windows 访问 WSL UNC 工作树路径失败，非产品路径验证

## 未闭合的发布边界

1. 本环境无法完成 Windows 原生 Wails/NSIS 构建、安装器升级 smoke 和 Windows Go 测试；交给 `.github/workflows/ci.yml` 与 `release.yml`。
2. 未对真实目标主机执行 Apply；macOS application firewall 的不可自动验证限制、未签名 Windows 安装器和未 notarize macOS 产物仍是既有边界。
3. 外部验证参数目前证明的是 TCP 可达性，不是加密身份认证；它不能替代控制电脑上的 SSH 主机指纹核对。

发布前必须以干净 checkout 在 Windows runner 上重跑原生 gates，并确认没有任何真实 profile、journal、host identity 或 secret 进入 release assets。
