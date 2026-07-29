package launchpad

import (
	"encoding/json"
	"regexp"
	"strings"
)

var (
	ipv4Pattern         = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	ipv6Pattern         = regexp.MustCompile(`(?i)(?:\b[0-9a-f]{1,4}:){2,}[0-9a-f:]*\b`)
	windowsUserPattern  = regexp.MustCompile(`(?i)\b[A-Z]:\\Users\\[^\\\s"]+`)
	unixHomePattern     = regexp.MustCompile(`/(?:Users|home)/[^/\s"]+`)
	tokenPattern        = regexp.MustCompile(`(?i)\b(token|cookie|authkey|authorization|password|passwd|secret|credential)\s*[:=]\s*[^\s,;"]+`)
	tailscaleKeyPattern = regexp.MustCompile(`(?i)\btskey-auth-[A-Za-z0-9_+/=-]+`)
	keyCommentPattern   = regexp.MustCompile(`(?m)((?:ssh-[^\s]+|ecdsa-[^\s]+|sk-[^\s]+)\s+[A-Za-z0-9+/=]+)(?:\s+[^\r\n]+)`)
)

// RedactReport creates a shareable report without changing the machine-readable schema.
// It removes common device identity, paths, public-key comments, and credential-like values.
func RedactReport(report Report) Report {
	data, _ := json.Marshal(report)
	var value any
	if json.Unmarshal(data, &value) != nil {
		return report
	}
	value = redactValue(value)
	clean, _ := json.Marshal(value)
	var redacted Report
	if json.Unmarshal(clean, &redacted) != nil {
		return report
	}
	if redacted.Snapshot != nil {
		redacted.Snapshot.Hostname = "<redacted-host>"
		if redacted.Snapshot.TargetUser != "" {
			redacted.Snapshot.TargetUser = "<redacted-user>"
		}
		if redacted.Snapshot.Tailscale.IP != "" {
			redacted.Snapshot.Tailscale.IP = "<redacted-ip>"
		}
	}
	if redacted.Plan != nil {
		for index := range redacted.Plan.Actions {
			if redacted.Plan.Actions[index].Operation == "configure_keys" {
				redacted.Plan.Actions[index].Command = []string{"<redacted-key-install-command>"}
			}
		}
	}
	return redacted
}

func redactValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			lower := strings.ToLower(key)
			sensitiveName := strings.Contains(lower, "token") || strings.Contains(lower, "cookie") || strings.Contains(lower, "privatekey") ||
				strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "credential") ||
				strings.Contains(lower, "authorization") || strings.Contains(lower, "authkey")
			if _, isString := item.(string); sensitiveName && isString {
				typed[key] = "<redacted>"
			} else {
				typed[key] = redactValue(item)
			}
		}
		return typed
	case []any:
		for index := range typed {
			typed[index] = redactValue(typed[index])
		}
		return typed
	case string:
		return redactText(typed)
	default:
		return value
	}
}

// redactText strips credential material, device identity, and public-key
// comments from free-form text such as captured command output.
func redactText(text string) string {
	text = ipv4Pattern.ReplaceAllString(text, "<redacted-ip>")
	text = ipv6Pattern.ReplaceAllString(text, "<redacted-ip>")
	text = windowsUserPattern.ReplaceAllString(text, `C:\Users\<redacted-user>`)
	text = unixHomePattern.ReplaceAllString(text, "/home/<redacted-user>")
	text = tokenPattern.ReplaceAllString(text, "$1=<redacted>")
	text = tailscaleKeyPattern.ReplaceAllString(text, "<redacted-tailscale-auth-key>")
	text = keyCommentPattern.ReplaceAllString(text, "$1 <redacted-comment>")
	if strings.Contains(text, "PRIVATE KEY") {
		return "<redacted-private-key-material>"
	}
	return text
}

func redactCredentialText(text, credential string) string {
	if credential = strings.TrimSpace(credential); credential != "" {
		text = strings.ReplaceAll(text, credential, "<redacted-tailscale-auth-key>")
	}
	return redactText(text)
}
