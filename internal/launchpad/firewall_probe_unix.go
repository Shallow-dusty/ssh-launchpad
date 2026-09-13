package launchpad

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

func probeUnixFirewall(ctx context.Context, port int) (FirewallState, error) {
	if _, err := exec.LookPath("ufw"); err == nil {
		out, err := runCommand(ctx, 8*time.Second, "ufw", "status", "verbose")
		if err != nil {
			return FirewallState{Provider: "ufw"}, err
		}
		return parseUFWInventory(string(out), port)
	}
	if _, err := exec.LookPath("firewall-cmd"); err == nil {
		return probeFirewalld(ctx, port)
	}
	return FirewallState{}, fmt.Errorf("no supported firewall provider was detected")
}

var ufwRule = regexp.MustCompile(`^\s*(\d+)(?:(?:-|:)(\d+))?(?:/tcp)?(?:\s+\(v6\))?\s+ALLOW(?:\s+IN)?\s+(.+?)\s*$`)

func parseUFWInventory(out string, port int) (FirewallState, error) {
	s := FirewallState{Provider: "ufw", Enabled: strings.Contains(strings.ToLower(out), "status: active")}
	fail := func() (FirewallState, error) {
		return s, fmt.Errorf("UFW contains an unsupported rule or unreadable default policy; review application profiles, raw rules and incoming exposure before Apply")
	}
	lower := strings.ToLower(out)
	if !s.Enabled {
		return s, fmt.Errorf("UFW is inactive")
	}
	if !strings.Contains(lower, "deny (incoming)") && !strings.Contains(lower, "reject (incoming)") {
		return fail()
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "Status:") || strings.HasPrefix(line, "Logging:") || strings.HasPrefix(line, "Default:") || strings.HasPrefix(line, "New profiles:") || strings.HasPrefix(line, "To ") || strings.HasPrefix(line, "--") {
			continue
		}
		// Outgoing rules do not open inbound SSH. Reject/deny rules can interfere
		// with reachability: do not certify them as successful allows.
		if strings.Contains(line, "ALLOW OUT") || strings.Contains(line, "DENY OUT") || strings.Contains(line, "REJECT OUT") {
			continue
		}
		match := ufwRule.FindStringSubmatch(line)
		if len(match) != 4 {
			return fail()
		}
		if !portSpecIncludes(match[1], match[2], port) {
			continue
		}
		scope := strings.TrimSpace(match[3])
		s.Ports = []int{port}
		s.Scopes = append(s.Scopes, scope)
		if match[2] != "" {
			s.PortRangeRules = append(s.PortRangeRules, line)
		}
		if broadFirewallScope(scope) || strings.HasPrefix(strings.ToLower(scope), "anywhere") {
			s.BroadExposure = true
		}
	}
	s.Checked = true
	return s, nil
}

func probeFirewalld(ctx context.Context, port int) (FirewallState, error) {
	s := FirewallState{Provider: "firewall-cmd"}
	query := func(args ...string) (string, error) {
		out, err := runCommand(ctx, 8*time.Second, "firewall-cmd", args...)
		return strings.TrimSpace(string(out)), err
	}
	fail := func(detail string, err error) (FirewallState, error) {
		return s, fmt.Errorf("firewalld inventory is incomplete/unsupported (%s): %v", detail, err)
	}
	zones, err := query("--get-active-zones")
	if err != nil {
		return fail("active zones", err)
	}
	active := []string{}
	for _, line := range strings.Split(zones, "\n") {
		if line != "" && line[0] != ' ' && line[0] != '\t' {
			active = append(active, strings.TrimSpace(line))
		}
	}
	zone, err := query("--get-default-zone")
	if err != nil || len(active) != 1 || active[0] != zone {
		return fail("exactly one active default zone is required", err)
	}
	for _, args := range [][]string{{"--get-active-policies"}, {"--direct", "--get-all-rules"}, {"--direct", "--get-all-passthroughs"}} {
		value, err := query(args...)
		if err != nil || value != "" {
			return fail("policy/direct rules require manual review", err)
		}
	}
	target, err := query("--zone="+zone, "--get-target")
	if err != nil || (target != "default" && target != "DROP" && target != "REJECT") {
		return fail("zone target", err)
	}
	// Services, source ports and forwarding need their own adapters. Do not
	// silently ignore them, including firewalld's stock broad ssh service.
	for _, flag := range []string{"--list-services", "--list-protocols", "--list-source-ports", "--list-forward-ports"} {
		value, err := query("--zone="+zone, flag)
		if err != nil || value != "" {
			return fail(flag+" must be empty in this supported adapter", err)
		}
	}
	ports, err := query("--zone="+zone, "--list-ports")
	if err != nil {
		return fail("ports", err)
	}
	for _, value := range strings.Fields(ports) {
		spec, ok := strings.CutSuffix(value, "/tcp")
		if !ok {
			continue
		}
		start, end, exact, ok := parsePortSpec(spec)
		if !ok {
			return fail("port syntax", nil)
		}
		if port >= start && port <= end {
			s.Ports = []int{port}
			s.BroadExposure = true
			if !exact {
				s.PortRangeRules = append(s.PortRangeRules, value)
			}
		}
	}
	rich, err := query("--zone="+zone, "--list-rich-rules")
	if err != nil {
		return fail("rich rules", err)
	}
	// Reload would replace runtime state. Refuse to mutate while persistent
	// and runtime inventories differ, including unrelated services/ports.
	for _, flag := range []string{"--list-services", "--list-protocols", "--list-source-ports", "--list-forward-ports", "--list-ports", "--list-rich-rules", "--get-target"} {
		runtimeValue, runErr := query("--zone="+zone, flag)
		persistentValue, diskErr := query("--permanent", "--zone="+zone, flag)
		if runErr != nil || diskErr != nil || runtimeValue != persistentValue {
			return fail("runtime/persistent drift: "+flag, diskErr)
		}
	}
	parsed, err := parseRichRules(rich, port)
	if err != nil {
		return fail("rich rule syntax/action", err)
	}
	s.Ports = append(s.Ports, parsed.Ports...)
	s.Scopes = parsed.Scopes
	s.BroadExposure = s.BroadExposure || parsed.BroadExposure
	s.PortRangeRules = append(s.PortRangeRules, parsed.PortRangeRules...)
	s.Checked = true
	s.Enabled = true
	return s, nil
}

var supportedRichRule = regexp.MustCompile(`^rule family="(ipv4|ipv6)" source address="([^"]+)" port port="([^"]+)" protocol="tcp" accept$`)

func parseRichRules(out string, port int) (FirewallState, error) {
	s := FirewallState{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := supportedRichRule.FindStringSubmatch(line)
		if len(m) != 4 {
			return s, fmt.Errorf("unsupported rich rule: %s", line)
		}
		start, end, exact, ok := parsePortSpec(m[3])
		if !ok {
			return s, fmt.Errorf("invalid rich rule port")
		}
		if port < start || port > end {
			continue
		}
		s.Ports = []int{port}
		s.Scopes = append(s.Scopes, m[2])
		s.BroadExposure = s.BroadExposure || broadFirewallScope(m[2])
		if !exact {
			s.PortRangeRules = append(s.PortRangeRules, line)
		}
	}
	return s, nil
}
