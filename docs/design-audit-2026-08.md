# SSH Launchpad 深度审核与设计（2026-08）

> **实施状态（2026-08-01）**：本报告的全部 P0/P1/P2 项已落地，分 7 个提交
> 完成（布局修复 → 轮询与诚实失败 → 高级页自动生效 → 向导重构 → 视觉收敛
> → 引擎安全收敛 → 文档收尾）。唯一有意偏差：R2 以“路由提示 + Apply 内权威
> 校验”替代原定的直接删除 replan，因为提权路由本身需要 no-Changes/需提权
> 两个信号，权威 digest 校验仍在 engine.Apply 内完成，fail-closed 语义不变。

审核范围：交互逻辑、说明性内容、UI/UX、安全过度设计残留。
方法：通读 Go 引擎 / Wails 桥 / CLI / 前端全部源码，并以 mock 模式实际运行
全部界面截图核对（Playwright，1280×900 + 390×844，zh-CN/en，亮暗两色）。

结论先行：**后端引擎是健康的，病灶集中在"产品层"**——前端状态机、文案纪律、
视觉系统。安全侧的过度设计已在 v0.2.3 后收敛，仍有 2 处残留（journal 自校验、
GUI 三次 replan），另有几处"合理但应降级为逃生口"的复杂度。

---

## 一、交互逻辑（核心病灶）

信息架构的根本问题：**向导是按系统阶段（Check/Plan/Apply/Verify）组织的，
而不是按用户任务组织的**。引擎的四个阶段直接漏出成了界面的四个步骤，
导致"说明"比"行动"多。

### P0 流程缺陷（真 bug）

1. **检测到公钥却不默认选中，主按钮必报错**
   `views.ts:renderRecommendationStep` 列出检测到的公钥但 radio 不勾选；
   `main.ts:useGuidedMode` 校验失败 → "还需要控制电脑的公钥"。
   新手视角：界面明明显示了密钥，点"使用推荐设置"却被训斥。
   实测截图 04 复现。**修复**：唯一检测 key 默认选中；多 key 默认选第一项；
   未选时主按钮 disabled 并就近提示，而不是点击后报错。

2. **Plan 失败 = 无限 Loading，错误被吞**
   `useGuidedMode` 的 catch 设置 `installState="failed"` 但 `planReport`
   为空；`renderInstallStep` 里 `if (!plan) return planLoading` —— 永远转圈，
   错误信息无处可见。**修复**：plan 失败渲染失败态（错误 + 返回按钮）。

3. **安装轮询每 500ms 整页 innerHTML 重绘**
   `main.ts:pollElevatedJob` 无条件 `renderPage()` → 滚动位置丢失、文本选择
   被打断、焦点被抢、`aria-live` 反复轰炸。**修复**：仅当
   `events.length` 或 `state` 变化时重绘；或进度区局部 DOM 更新。

4. **Verify 失败时拿 Apply 的旧报告冒充**
   `runVerify` catch 里 `state.verifyReport = state.activeJob?.report` ——
   测试页随后展示的是 Apply 时刻的过期快照，却按"刚才的验证结果"呈现。
   **修复**：verify 失败就显示失败，不回填。

5. **repair 模式 = setup 模式换皮**
   `renderWizard` 中 mode 只改 eyebrow 文案。"连不上"的用户被塞进同一个
   四步安装漏斗。i18n 里 `repairIntro`（"先只读检查，只显示需要修复的项目"）
   存在但**从未被渲染**——死字符串证实了设计意图没落地。
   **修复**：repair 应以"诊断结果 + 针对性修复项"为主界面（见第五节）。

### P1 决策点设计缺失

6. **"使用推荐设置" vs "仅在局域网使用"两个并排按钮，无对比、无后果说明**
   新手无法判断。应改为单个选择控件（默认 Tailscale），LAN 作为次级选项，
   附一行后果："局域网内任何设备都可尝试连接此端口"。

7. **"还差 4 步"不可行动**：哪 4 步？`missingCount` 数的是 11 项技术断言，
   且与下一步 plan 的"5 个项目"数字口径不一致（4 ≠ 5，用户无法核对）。
   检查页应列出缺项清单（OpenSSH 未安装 / 服务未运行 / 防火墙未放行 /
   安全网络未连接），而不是一个无法解释的数字。

8. **检查完成后的 banner 正文是 checkBody**（"只读取系统……不会改动电脑"）——
   检查都跑完了还在解释检查是什么。结果位应说结果。

9. **高级模式的"保存"心智混乱**：表单需要点"使用这些高级设置"才生效，
   但"只读检查/生成变更方案/导出配置/导出信息卡"内部**隐式调用
   `saveAdvanced()`**。用户以为没保存，实际被存了。要么 onChange 自动生效
   并去掉保存按钮，要么显式"未保存更改"标记；隐式副作用是最差选项。

10. **错误用 toast 呈现，2.8 秒自动消失**：安装失败的长说明一闪而过。
    错误应持久（失败态卡片或 dialog），toast 只用于"已复制/已导出"这类
    瞬时确认。

### P2 细节

11. 返回按钮文字是步骤名（"推荐方案"）而非"上一步"——新手认知负担。
12. 步骤条不可点击（可接受，但配合 11 更应有明确的"上一步"）。
13. "检查新版本"点击后直接弹浏览器下载页，无中间确认（当前文案说"只打开
    下载页"，行为一致但突兀）。
14. 向导内语言切换重建整个 shell，进行中的确认弹窗内容会丢失。
15. `rollbackLast` 用原生 `confirm()`，Wails WebView2 下观感差且风格不统一。

---

## 二、说明性内容入侵产品（文案病）

典型 GPT 症状：**把"解释"当"界面"**。实锤：

1. **同一件事讲三遍（plan 步骤）**：action 列表（带理由）→ access-summary
   （将安装/将打开/谁能连接）→ 确认弹窗（再次完整列出 action）。
   收敛为一处：确认弹窗即唯一完整清单，plan 页只给摘要 + 风险。

2. **推荐页 info-note 逐字重复正文**：`tailscaleNote` 在 Tailscale online 时
   直接取 `t("recommendationBody")`（代码实锤），同一段落屏上出现两次。

3. **每个 action 的理由是同一句零信息量废话**："当前状态与推荐设置不同，
   需要完成这一项。" ×4。planner 里 `action.Reason` 有具体的英文原因，
   前端却没用它。要么翻译真实 reason，要么删掉理由行。

4. **测试页三重免责**：banner（"本机检查通过……进行一次真实连接"）+
   boundary-note（"本机绿灯不等于跨设备成功……"）+ info-note（"下一步：
   到控制电脑粘贴这条命令……"）—— 三段话讲同一件事。合并成一条"下一步"
   指引。

5. **私钥概念讲三遍**：首页 role-card、推荐页 keyExplain、选中后
   keySelected 提示，都在讲"私钥不离开控制电脑"。首次讲一次即可。

6. **常驻红 banner 展示永久状态**：高级页底部未签名声明用 danger 红色
   常驻 —— 用最高告警等级展示常态信息，产生"狼来了"脱敏。降为中性说明。
   首页脚部"无遥测 + 未签名"挤一行也是文档内容进了产品 chrome。

7. **"简单结论" eyebrow 装饰性滥用**：每个 banner 顶上都有一枚，纯噪音。

8. **死文案**：`repairIntro`、`close` 两个 i18n key 从未被引用。

原则：**渐进式披露**。解释性内容收进 (?) 帮助气泡 / "为什么？"折叠 /
离线帮助文档；界面只保留当下决策所需信息。每个概念全产品只讲一次，
讲在它产生影响的决策点上。

---

## 三、UI/UX（截图实锤）

### 3.1 坏的（布局/标记 bug，非审美）

| # | 问题 | 位置 | 证据 |
| --- | --- | --- | --- |
| B1 | plain-card 标签值粘连："这台电脑HOME-PC" | `views.ts:plainCard` + `styles.css` | `.plain-card` 无 grid；`small/strong` 未 `display:block` |
| B2 | connection-facts 标记与 CSS 选择器不匹配 | `views.ts:renderTestStep` 用 `<span><small>…<b>`，CSS 写 `.connection-facts > div` / `strong` | 边框、块级、字号全部失效（"主机HOME-PC"） |
| B3 | 测试页端口标签错为"将打开22" | 复用 `t("willOpen")` | 应为"端口" |
| B4 | connection-visual 是空盒子：两个悬浮小图标 + 无样式的 `<span></span>`（应为连线） | `renderTestStep` | 看起来像没做完 |
| B5 | 推荐任务卡全宽高亮失效 | `views.ts:taskCard` 从不给 button 加 `recommended` class | `.task-card.recommended{grid-column:1/-1;…}` 死 CSS，设计意图（推荐路径主视觉）丢失 |
| B6 | 死 CSS / 死 i18n | `.form-grid .field.full`、`repairIntro`、`close` | 死代码 |

### 3.2 设计系统问题（审美方向）

1. **字体尺度失控**：`h2 clamp(1.5rem,3vw,2.15rem)` 用于每张卡片标题 →
   满屏巨型标题，无层级。"先分清两台电脑"这种说明卡与主任务标题同级。
2. **模板感**：22px 圆角 + backdrop-blur 毛玻璃 + 大阴影 + 渐变按钮 =
   典型"AI 生成后台"审美。安全工具应克制：圆角 10–12px、去 blur、弱阴影、
   按钮纯色。
3. **色彩语义稀释**：danger 红用于常驻提示；good 绿大块用于装饰
   （connection-visual）；warn 黄用于"还差 4 步"这种**正常流程状态**——
   用户走正常路径看到的第一个 banner 就是警告色。正常未完成 = 信息，不是警告。
4. **图标手绘不一致**：`doorIcon` 小尺寸不可辨认（截图里像个方框）。
   建议整体换 Lucide/Feather（MIT，风格与现有 stroke 一致，直接替换 SVG
   path 即可，成本极低）。
5. **对齐**：role-card / personal-card-strip 图标垂直居中、内容顶部对齐，
   视觉上图标"沉底"；wizard-actions `justify-between` 让中间按钮（LAN）
   悬在怪异位置。
6. **移动端**：role-pair 折行难看；主战场是 Windows 桌面窗口，优先级低，
   但 Wails 窗口可拉窄，响应式仍要保住基本盘。

---

## 四、安全过度设计残留

已纠正的三处（单 `sshd -T`、去 SID 检查、简化进程锁）方向正确。残留：

### R1. Journal 自校验 digest —— 安全剧场，且有害【建议降级】
`executor.go:writeJournalAtomic` 每次写盘自算 SHA-256，`readJournal`
校验不匹配则**硬失败**。自算 digest 对防篡改零价值（能写 journal 的人能
重算 digest，算法就在二进制里），唯一作用是检测意外损坏——而 atomic
write 已经覆盖了这个场景。代价是**恢复路径多了一个硬失败模式**：
版本迁移、半写坏、人工编辑过的 journal 都会让回滚拒绝执行。
恢复工具因为自校验而拒绝恢复，本末倒置。
**建议**：digest 校验降级为 warning + 继续（或交互确认），或直接删除
digest 字段（保留 atomic write）。这是"最佳实现的边界"问题：防御型编程
防错了对象。

### R2. GUI 一次安装跑 3 次完整 Probe+Plan —— 性能型过度设计【建议优化】
向导 plan 步骤 1 次、`BeginElevatedApply` 复核 1 次、`engine.Apply`
内部 digest 校验再 1 次。Windows 上每次 probe 都是一串 PowerShell 子进程
（sshd -T、Get-WindowsCapability、防火墙枚举），点"开始安全安装"到 UAC
弹出之间明显卡顿。plan digest 绑定对 **CLI 的 plan→apply 分离场景**合理
（plan.json 落盘后被审阅再执行，必须防状态漂移）；但 GUI 是同进程连续流，
`BeginElevatedApply` 那次 replan 只是"提前报错"，删掉它，让 `engine.Apply`
统一报 digest 不匹配即可（错误文案已经现成）。

### R3. 合理但应定位为逃生口（保留，不动）
- `--schedule-risky` / `--external-verify-target` / `ConfirmHighRisk`：
  CLI 专属，GUI 恒传 false。对新手产品属于 enterprise 思维，但作为
  power-user 逃生通道成立。建议 README 淡化，GUI 永不暴露。
- elevation protocol（hash-bound request、pre-create、同目录校验、
  reparse 拒绝、一次性 consume、启动 prune）：因为有 authKey 落盘这个
  **真实资产**，复杂度有对应威胁，**不算过度**，保持。
- RedactReport 启发式脱敏：文档已声明启发式边界，保持。

### R4. 值得一提的非过度设计问题
- 前端 `friendlyError` 用正则匹配错误字符串分类（checksum/network），
  脆弱；建议后端返回结构化错误码。P2。
- 浏览器 mock 模式的公钥校验 fallback（atob 长度 > 16）很弱，但桌面端有
  Go 侧 `ValidatePublicKey` 兜底，mock 仅预览用，可接受，建议加注释。

### 后端值得肯定（避免误伤）
引擎/表面分离、marker 延迟物化 authKey、verify fail-closed、self-cut
默认阻止、plan 只读、退出码契约稳定——这些都是正确设计，审核中未发现
需要推翻的部分。

---

## 五、重设计方案

### 5.1 信息架构：按用户任务，不按系统阶段

用户任务只有三个，首页三卡不变，但内部重构：

**任务 A：让我能连上这台电脑（首次设置）→ 两屏制**
- 屏 1「方案」（自动检查后进入）：
  - 检查结果收敛成一行状态（"可以直接设置" / "需要先装 2 个组件"）；
  - 网络模式：单个选择控件，默认 Tailscale（推荐徽标），LAN 次级，
    各附一行后果说明；
  - 公钥：检测到的默认选中，粘贴/导入/生成收进次级操作；
  - 变更预览：一句话（"将安装 OpenSSH、仅允许你的 Tailscale 网络访问
    端口 22"）+ 可展开明细。
  - 主按钮「开始设置」→ 弹一次确认（唯一完整清单）→ UAC。
- 屏 2「完成」：进度（增量渲染）→ 结果 + 连接指引（命令复制、指纹核对、
  "现在去另一台电脑"清单）。说明文字只在这一屏保留一次。

**任务 B：连不上了（诊断修复）→ 报告即主页**
- 进入即跑 Check，结果页 = 问题清单（每项：现象 → 一个"修复"按钮），
  或"全部修复"一键。没有"推荐方案/安全安装"这些阶段语言。
- 这正是 `repairIntro` 死文案描述的原设计，把它落地。

**任务 C：信息卡 / 高级**：保持，修复 9（隐式保存）。

引擎四阶段（Check/Plan/Apply/Verify）保留为**内部实现和 CLI 契约**，
不再 1:1 漏出到 GUI 步骤条。

### 5.2 文案纪律

- 每个概念全产品只讲一次，讲在决策点上；
- banner 只说两件事：现在什么状态 + 下一步做什么；
- 删除零信息量句子（评判标准：把这句话删掉，用户是否损失任何决策依据）；
- 解释性内容全部进 (?)/折叠/离线帮助；
- 错误持久呈现，toast 只做瞬时确认。

### 5.3 视觉方向（设计 token 级）

| 项 | 现状 | 建议 |
| --- | --- | --- |
| 圆角 | 22px | 12px（卡片）/ 8px（控件） |
| 毛玻璃 | backdrop-blur 24px | 删除（桌面工具不需要，且省 GPU） |
| 阴影 | 0 18px 55px | 0 1px 2px + 0 6px 20px，透明度减半 |
| 标题层级 | h2 2.15rem 到处都是 | 页标题 1.6rem / 卡片标题 1.05rem / 正文 .925rem |
| 语义色 | 红/黄/绿到处用 | 三态收敛：信息=中性蓝灰、成功=绿、失败=红；黄仅用于"需要用户决定的注意项" |
| 图标 | 手绘 | Lucide（MIT），同 stroke 风格平替 |
| 推荐卡 | 与普通卡相同（bug） | 恢复全宽 + 主色描边，成为视觉焦点 |

### 5.4 工程清单（按优先级）

**P0（交互正确性）**
1. 检测公钥默认选中 / 未选禁用主按钮（修 1）
2. plan 失败态渲染（修 2）
3. 轮询改增量渲染（修 3）
4. verify 失败不回填 apply 报告（修 4）
5. plain-card / connection-facts / 端口标签 / connection-visual 四处修复（B1–B4）

**P1（产品形态）**
6. 首次设置改两屏制；repair 改报告即主页（落地 repairIntro）
7. 网络模式改单选控件 + 后果说明
8. 检查页缺项清单替代"还差 N 步"
9. 高级模式保存心智（自动生效 or 显式脏标记）
10. 文案三遍收敛 + 死文案/死 CSS 清理 + Lucide 图标

**P2（打磨与安全收敛）**
11. journal digest 降级为 warn-and-continue（R1）
12. 删 `BeginElevatedApply` 多余 replan（R2）
13. 错误持久化呈现；原生 confirm 换 dialog
14. 未签名声明降中性；语义色收敛
15. 结构化错误码替代字符串正则匹配

---

## 附：审核证据

- 截图（mock 模式全界面，zh/en × 亮/暗 × 桌面/移动）：本次审核在
  `/tmp/shots/` 采集，属临时证据，按仓库约定不入库。
- 死代码检测脚本输出：`repairIntro`、`close`、`.field.full`、
  `.task-card.recommended`。
- 后端逐文件阅读：`engine.go`（三次 replan 路径）、`executor.go`
  （journal digest）、`elevation/protocol.go`（handoff 边界）、
  `planner.go`（真实 reason 未被前端使用）。
