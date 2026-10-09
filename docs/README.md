# 文档导航

## 先看哪份

| 你要做什么 | 入口 | 内容归属 |
| --- | --- | --- |
| 下载、使用产品 | [项目 README](../README.md) | 快速开始与用户说明 |
| 接手项目、看现在做到哪里 | [STATUS](../STATUS.md) | 当前版本、已验证范围、未闭合边界 |
| 决定下一步做什么 | [ROADMAP](../ROADMAP.md) | 待办与优先级，不重复已完成版本史 |
| 改代码、跑测试、找目录 | [开发说明](development.md) | 工具链、目录地图、产物位置 |
| 准备发布 | [发布清单](release-verification.md) | 发布流程与检查项 |
| 查某版改了什么 | [CHANGELOG](../CHANGELOG.md) | 已完成的版本变化 |

现行合同以代码和主题文档为准；STATUS 中的日期、提交与验证范围决定哪些
证据仍适用。日期记录与历史审计不自动等于当前版本已验收。

## 使用与恢复

- [中文离线帮助](offline-help.zh-CN.md) / [English offline help](offline-help.en.md)
- [故障排查与恢复](troubleshooting.md)
- [下载策略与依赖适配边界](network-download-strategy.md)
- [离线依赖包](offline-pack.md)

## 架构与维护

- [架构与执行合同](architecture.md)
- [平台支持与验证边界](platform-support.md)
- [威胁模型](threat-model.md) / [安全问题报告](../SECURITY.md)
- [贡献约定](../CONTRIBUTING.md)

## 当前候选版的日期记录：`records/`

记录当时的发现、改动和证据；后续验证追加独立日期记录，并更新 STATUS 摘要。
不要把报告中的“当时未完成”直接当作今天的待办。

- [2026-09-13 安全修复与验证](records/audit-2026-09.md)
- [2026-09-13 前端设计与浏览器验证](records/ui-refinement-2026-09.md)
- [2026-09-15/16 真机验收](records/acceptance-2026-09.md)
- [2026-10-08/09 代码收敛、安全升级与本地回归](records/code-consolidation-2026-10-09.md)

## 已被替代的材料：`90.Archive/`

- [八月审计、设计审核和早期界面截图](90.Archive/01.2026-08-audits/ARCHIVE_NOTE.md)
- [整理前 STATUS / ROADMAP 原文](90.Archive/02.pre-organization-baseline/ARCHIVE_NOTE.md)

归档保留来源、原因与活跃替代，不作为现行实施规范。已发布版本的 tag notes
仍放在 `.github/release-notes-<tag>.md`，供发布工作流读取，不搬进归档。

## 原始产物不进文档

本地日志、复现脚本、测试报告与敏感 journal 放在被忽略的 `build/audit/`；
浏览器结果在 `frontend/test-results/`。历史原始证据不清空，也不因整理而提交。
完整路径约定见 [开发说明](development.md#repository-map)。
