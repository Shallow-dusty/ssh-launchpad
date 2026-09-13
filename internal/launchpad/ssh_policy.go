package launchpad

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Inspect configuration independently of sshd -T: the global dump omits Match
// policy. Unknown syntax fails closed rather than guessing connection semantics.
func inspectSSHPolicy(path string, platform Platform) error {
	return inspectSSHFile(path, filepath.Dir(path), platform, map[string]bool{}, false, 0)
}

func inspectSSHFile(path, baseDir string, platform Platform, seen map[string]bool, included bool, depth int) error {
	path = filepath.Clean(path)
	if depth > 16 || seen[path] {
		return fmt.Errorf("recursive SSH Include: %s", path)
	}
	seen[path] = true
	defer delete(seen, path)
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("read SSH policy: %w", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	adminMatch := false
	for scanner.Scan() {
		line := strings.TrimSpace(strings.SplitN(scanner.Text(), "#", 2)[0])
		if line == "" {
			continue
		}
		line = strings.Replace(line, "=", " ", 1)
		fields := strings.Fields(line)
		key := strings.ToLower(fields[0])
		if key == "match" {
			// The sole supported Match is the stock Windows admin key redirection.
			if platform == PlatformWindows && len(fields) == 3 && strings.EqualFold(fields[1], "group") && strings.EqualFold(fields[2], "administrators") {
				adminMatch = true
				continue
			}
			return fmt.Errorf("unsupported SSH Match policy in %s; review connection-specific authentication before Apply", path)
		}
		if adminMatch {
			if key == "authorizedkeysfile" && len(fields) == 2 && strings.EqualFold(strings.ReplaceAll(fields[1], `\`, "/"), "__PROGRAMDATA__/ssh/administrators_authorized_keys") {
				continue
			}
			return fmt.Errorf("unsupported directive inside SSH Match in %s", path)
		}
		if key == "port" && included {
			return fmt.Errorf("SSH Port in Include %s requires manual consolidation into sshd_config", path)
		}
		if key == "listenaddress" {
			return fmt.Errorf("custom SSH ListenAddress in %s requires manual verification of all listening endpoints", path)
		}
		if key != "include" {
			continue
		}
		if len(fields) < 2 {
			return fmt.Errorf("empty SSH Include in %s", path)
		}
		for _, pattern := range fields[1:] {
			if strings.ContainsAny(pattern, "\"'%") {
				return fmt.Errorf("unsupported quoted/tokenized SSH Include in %s", path)
			}
			if !filepath.IsAbs(pattern) {
				pattern = filepath.Join(baseDir, pattern)
			}
			paths, err := filepath.Glob(pattern)
			if err != nil {
				return err
			}
			for _, child := range paths {
				if err := inspectSSHFile(child, baseDir, platform, seen, true, depth+1); err != nil {
					return err
				}
			}
		}
	}
	return scanner.Err()
}

func hasStockAdminMatch(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(strings.ToLower(strings.TrimSpace(line)))
		if len(fields) == 3 && fields[0] == "match" && fields[1] == "group" && fields[2] == "administrators" {
			return true
		}
	}
	return false
}

func sshConfigPath(platform Platform) string {
	if platform == PlatformWindows {
		return filepath.Join(programDataDir(), "ssh", "sshd_config")
	}
	return "/etc/ssh/sshd_config"
}

func parseConfiguredSSHPorts(out []byte) []int {
	var ports []int
	for _, line := range strings.Split(string(out), "\n") {
		if p := parseConfiguredSSHPort([]byte(line)); p > 0 && !containsInt(ports, p) {
			ports = append(ports, p)
		}
	}
	return ports
}
