package launchpad

import (
	"encoding/json"
	"regexp"
	"strings"
)

var (
	ipv4Pattern        = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	windowsUserPattern = regexp.MustCompile(`(?i)\b[A-Z]:\\Users\\[^\\\s"]+`)
	unixHomePattern    = regexp.MustCompile(`/(?:Users|home)/[^/\s"]+`)
	tokenPattern       = regexp.MustCompile(`(?i)\b(token|cookie|authkey|authorization)\s*[:=]\s*[^\s,;"]+`)
	keyCommentPattern  = regexp.MustCompile(`(?m)((?:ssh-[^\s]+|ecdsa-[^\s]+)\s+[A-Za-z0-9+/=]+)(?:\s+[^\r\n]+)`)
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
		if redacted.Snapshot.Tailscale.IP != "" {
			redacted.Snapshot.Tailscale.IP = "<redacted-ip>"
		}
	}
	return redacted
}

func redactValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "token") || strings.Contains(lower, "cookie") || strings.Contains(lower, "privatekey") {
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
		text := ipv4Pattern.ReplaceAllString(typed, "<redacted-ip>")
		text = windowsUserPattern.ReplaceAllString(text, `C:\Users\<redacted-user>`)
		text = unixHomePattern.ReplaceAllString(text, "/home/<redacted-user>")
		text = tokenPattern.ReplaceAllString(text, "$1=<redacted>")
		text = keyCommentPattern.ReplaceAllString(text, "$1 <redacted-comment>")
		if strings.Contains(text, "PRIVATE KEY") {
			return "<redacted-private-key-material>"
		}
		return text
	default:
		return value
	}
}
