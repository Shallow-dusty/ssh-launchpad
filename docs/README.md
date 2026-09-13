# 文档导航

现行说明以仓库代码、[STATUS](../STATUS.md) 与下列主题文档为准。
历史审计记录的是当时的发现和验证，不是当前安全保证。

## 使用与恢复

- [项目介绍与快速开始](../README.md)
- [中文离线帮助](offline-help.zh-CN.md) / [English offline help](offline-help.en.md)
- [故障排查与恢复](troubleshooting.md)
- [下载策略与依赖适配边界](network-download-strategy.md)
- [离线依赖包](offline-pack.md)

## 架构与开发

- [架构与执行合同](architecture.md)
- [平台支持与验证边界](platform-support.md)
- [威胁模型](threat-model.md) / [安全问题报告](../SECURITY.md)
- [开发与测试](development.md)
- [发布验收清单](release-verification.md)
- [版本变更](../CHANGELOG.md)

## 本轮审计与设计

- [2026-09 安全修复与验证总结](audit-2026-09.md)
- [2026-09 前端设计与交互优化](ui-refinement-2026-09.md)

## 历史记录

以下文档保留原名与历史数据，不据此推断当前版本状态：

- [2026-08-23 工作树审计](audit-current-2026-08.md)
- [2026-08 设计审核](design-audit-2026-08.md)
- [设计审核 v2](design-audit-2026-08-v2.md)
- `images/v0.2-*.png`：早期界面参考，非当前版本验收截图。

运行日志、临时复现、浏览器截图与敏感 journal 不进入文档目录或提交。
本地验证输出存放于 `build/audit/` 或 `frontend/test-results/`。
