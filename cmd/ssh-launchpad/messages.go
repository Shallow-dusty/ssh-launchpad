package main

import (
	"fmt"
	"strings"
)

var zhMessages = map[string]string{
	"welcome":                 "SSH Launchpad 远程连接助手",
	"chooseTask":              "你想做什么？不用先了解 SSH 或配置文件。",
	"setupTask":               "让这台电脑可以被远程连接（推荐）",
	"repairTask":              "检查并修复远程连接",
	"checkTask":               "只检查，不做改动",
	"choicePrompt":            "请输入序号",
	"invalidChoice":           "没有识别这个选项，电脑没有改动。",
	"checking":                "正在检查这台电脑……",
	"ready":                   "已准备好",
	"checkFailed":             "检查没有完成，电脑没有改动。",
	"missingSteps":            "还差 %v 步",
	"computer":                "电脑",
	"recommendTitle":          "推荐方案",
	"recommendTailnet":        "优先只允许你的安全网络设备连接；若 Tailscale 不可用，会在安装前说明其他选择。",
	"alreadyReady":            "已经配置好；重复运行不会重复修改。",
	"willChange":              "确认后将做这些事：",
	"whoCanConnect":           "默认只允许安全网络中的设备连接，不会开放给整个互联网。",
	"selfCutBlocked":          "这些操作可能切断当前唯一远程连接，已自动阻止。请到目标机本地执行，或先准备第二条连接。",
	"applyPrompt":             "是否使用推荐设置并继续？输入“是”才会改动",
	"permissionPrompt":        "下一步需要系统管理员权限。系统会显示安全确认窗口。",
	"continuePrompt":          "继续",
	"permissionCancelled":     "你取消了管理员授权，没有继续改动。",
	"permissionDone":          "管理员步骤已完成。",
	"noChanges":               "已取消，电脑没有改动。",
	"keyTitle":                "谁来控制这台电脑？",
	"keyExplain":              "这里只需要“控制电脑”的公钥。私钥始终留在控制电脑上，绝不会复制或上传。",
	"foundKey":                "发现 %v 个现有公钥，已选择第一个。稍后可在高级模式更换。",
	"noKey":                   "没有发现公钥。你可以粘贴控制电脑的公钥，或在本机生成一对新密钥。",
	"pasteKey":                "粘贴控制电脑的公钥",
	"generateKey":             "安全生成新密钥（私钥只保存在本机）",
	"pastePrompt":             "粘贴以 ssh-ed25519 等开头的一整行公钥",
	"publicOnly":              "这里只接受公钥；不要粘贴、复制或上传私钥。",
	"privateExists":           "默认私钥文件已经存在，为避免覆盖已停止。请使用现有公钥或在高级模式选择其他位置。",
	"keygenFailed":            "生成密钥失败",
	"generatedKey":            "已生成密钥；私钥只保存在当前用户的 .ssh 目录。",
	"verifyNeedsOtherDevice":  "本机检查已完成。最终还需从另一台设备测试连接。",
	"copyCommand":             "在另一台电脑复制并运行：",
	"fingerprintWarning":      "首次连接请核对屏幕上的主机指纹；不要静默接受未知指纹。",
	"pressEnter":              "按 Enter 键关闭……",
	"alreadyRunning":          "另一个 SSH Launchpad 向导正在运行。请回到那个窗口，避免同时修改。",
	"yes":                     "是",
	"no":                      "否",
	"installSSH":              "安装 OpenSSH 远程连接服务",
	"configureSSH":            "使用安全设置配置远程连接",
	"configureKeys":           "允许选定控制电脑的公钥登录",
	"enableSSH":               "启动服务并在开机后自动启动",
	"configureFirewall":       "仅为端口 %v 添加受限防火墙规则",
	"installTailscale":        "安装可选的安全网络组件",
	"systemChange":            "完成必要的系统设置",
	"working":                 "正在处理",
	"completed":               "已完成",
	"rollingBack":             "正在恢复到执行前",
	"operationFailed":         "操作没有完成。已停止后续步骤；可重试或查看支持信息。",
	"checksumFailed":          "下载文件校验失败，已拒绝使用。请重新下载或换可信来源。",
	"networkFailed":           "网络暂时不可用。可检查代理、稍后重试，或使用已校验的离线包。",
	"unknownCommand":          "无法识别命令 %q",
	"rollbackRequiresJournal": "恢复操作需要 --journal 指定执行记录。",
	"profileHelp":             "JSON 或 YAML 高级配置文件",
	"outputHelp":              "报告文件路径；- 表示标准输出",
	"confirmHelp":             "确认执行 Apply",
	"selfCutHelp":             "允许可能切断当前远程连接的操作",
	"scheduleHelp":            "延迟执行高风险控制通道操作",
	"journalHelp":             "恢复记录目录",
	"externalHelp":            "由另一台电脑验证的 host:port",
	"updateAvailable":         "发现新的稳定版：",
	"upToDate":                "当前已是最新稳定版。不会静默升级系统组件。",
	"usage": `SSH Launchpad — 远程连接助手

直接双击或无参数运行：打开中文交互向导

常用命令：
  ssh-launchpad check                         只检查，不改动
  ssh-launchpad plan --profile profile.yaml   查看将要改什么
  ssh-launchpad apply --profile profile.yaml --yes
  ssh-launchpad verify --profile profile.yaml
  ssh-launchpad rollback --journal <记录文件>
  ssh-launchpad update                        只检查稳定版更新

语言与自动化：
  --lang auto|zh-CN|en     自动/中文/English（也可用 SSH_LAUNCHPAD_LANG）
  --non-interactive        CI/脚本模式，绝不等待输入
  --json                   机器输出保持稳定英文 JSON 字段

Check 和 Plan 严格只读；Verify 不提权；Apply 会明确确认并默认阻止切断当前唯一远程连接。`,
}

var enMessages = map[string]string{
	"welcome":                 "SSH Launchpad connection assistant",
	"chooseTask":              "What would you like to do? You do not need to know SSH or edit a profile.",
	"setupTask":               "Make this computer available for remote connection (recommended)",
	"repairTask":              "Check and repair remote connection",
	"checkTask":               "Check only; make no changes",
	"choicePrompt":            "Enter a number",
	"invalidChoice":           "That option was not recognized. No changes were made.",
	"checking":                "Checking this computer...",
	"ready":                   "Ready",
	"checkFailed":             "The check did not finish. No changes were made.",
	"missingSteps":            "%v steps remaining",
	"computer":                "Computer",
	"recommendTitle":          "Recommended setup",
	"recommendTailnet":        "Only your trusted network devices can connect by default. Alternatives are explained before installation if Tailscale is unavailable.",
	"alreadyReady":            "Already configured. Running again will not repeat changes.",
	"willChange":              "After confirmation, SSH Launchpad will:",
	"whoCanConnect":           "Allow trusted-network devices only by default; it will not expose the computer to the entire internet.",
	"selfCutBlocked":          "These actions might cut the only remote connection and were blocked. Run locally on the target or prepare a second connection.",
	"applyPrompt":             "Continue with recommended settings? Type \"yes\" to make changes",
	"permissionPrompt":        "The next step needs administrator permission. Your system will show a security prompt.",
	"continuePrompt":          "Continue",
	"permissionCancelled":     "Administrator permission was cancelled. No further changes were made.",
	"permissionDone":          "Administrator step completed.",
	"noChanges":               "Cancelled. No changes were made.",
	"keyTitle":                "Which computer will control this one?",
	"keyExplain":              "Only the controller computer's public key is needed. Its private key stays on the controller and is never copied or uploaded.",
	"foundKey":                "Found %v public keys and selected the first. Advanced mode can change it later.",
	"noKey":                   "No public key was found. Paste the controller's public key or safely generate a new key pair.",
	"pasteKey":                "Paste the controller computer's public key",
	"generateKey":             "Safely generate a key (private key stays here)",
	"pastePrompt":             "Paste the complete public-key line beginning with ssh-ed25519, for example",
	"publicOnly":              "Only a public key is accepted. Never paste, copy, or upload a private key.",
	"privateExists":           "The default private-key file already exists, so it was not overwritten. Use its public key or select another path in advanced mode.",
	"keygenFailed":            "Key generation failed",
	"generatedKey":            "Key generated. The private key stays in this user's .ssh directory.",
	"verifyNeedsOtherDevice":  "Local checks finished. Complete the final connection test from another device.",
	"copyCommand":             "Copy and run on the other computer:",
	"fingerprintWarning":      "On first connection, compare the host fingerprint shown on screen. Never silently accept an unknown fingerprint.",
	"pressEnter":              "Press Enter to close...",
	"alreadyRunning":          "Another SSH Launchpad wizard is running. Return to that window to avoid concurrent changes.",
	"yes":                     "yes",
	"no":                      "no",
	"installSSH":              "Install the OpenSSH remote connection service",
	"configureSSH":            "Configure remote access with secure settings",
	"configureKeys":           "Allow the selected controller public key",
	"enableSSH":               "Start the service and enable it at sign-in or boot",
	"configureFirewall":       "Add a restricted firewall rule for port %v",
	"installTailscale":        "Install the optional secure-network component",
	"systemChange":            "Complete a required system setting",
	"working":                 "Working",
	"completed":               "Completed",
	"rollingBack":             "Restoring the previous state",
	"operationFailed":         "The operation did not finish. Later steps were stopped; retry or open support details.",
	"checksumFailed":          "The downloaded file failed verification and was rejected. Download again or use another trusted source.",
	"networkFailed":           "The network is unavailable. Check the proxy, retry later, or use a verified offline bundle.",
	"unknownCommand":          "Unknown command %q",
	"rollbackRequiresJournal": "Rollback requires --journal with an execution record.",
	"profileHelp":             "JSON or YAML advanced profile",
	"outputHelp":              "report path, or - for standard output",
	"confirmHelp":             "confirm Apply",
	"selfCutHelp":             "allow an action that could cut the active connection",
	"scheduleHelp":            "schedule risky control-path work after a delay",
	"journalHelp":             "rollback journal directory",
	"externalHelp":            "controller-visible host:port for external verification",
	"updateAvailable":         "New stable release:",
	"upToDate":                "You have the latest stable version. System components are never silently upgraded.",
	"usage": `SSH Launchpad - remote connection assistant

Double-click or run without arguments: open the interactive wizard

Common commands:
  ssh-launchpad check                         check only; no changes
  ssh-launchpad plan --profile profile.yaml   review exact changes
  ssh-launchpad apply --profile profile.yaml --yes
  ssh-launchpad verify --profile profile.yaml
  ssh-launchpad rollback --journal <record>
  ssh-launchpad update                        check the stable channel only

Language and automation:
  --lang auto|zh-CN|en     auto/Chinese/English (or SSH_LAUNCHPAD_LANG)
  --non-interactive        never wait for input in CI/scripts
  --json                   stable English JSON field names

Check and Plan are strictly read-only. Verify never elevates. Apply confirms exact changes and blocks active-connection self-cut by default.`,
}

func tr(key string, values ...any) string {
	messages := enMessages
	if currentLanguage == langZH {
		messages = zhMessages
	}
	value, ok := messages[key]
	if !ok {
		value = enMessages[key]
	}
	if len(values) > 0 {
		return fmt.Sprintf(value, values...)
	}
	return value
}

func languageCompleteness() []string {
	var missing []string
	for key := range zhMessages {
		if strings.TrimSpace(enMessages[key]) == "" {
			missing = append(missing, "en:"+key)
		}
	}
	for key := range enMessages {
		if strings.TrimSpace(zhMessages[key]) == "" {
			missing = append(missing, "zh-CN:"+key)
		}
	}
	return missing
}
