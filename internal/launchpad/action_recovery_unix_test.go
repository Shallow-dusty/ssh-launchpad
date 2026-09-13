//go:build !windows

package launchpad

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedConfigFailureRestoresExactPriorFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sshd_config")
	original := []byte("# retain comments\nPort 22\nPort 2200\nPasswordAuthentication yes\n")
	if err := os.WriteFile(path, original, 0640); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(dir, "sshd")
	if err := os.WriteFile(shim, []byte("#!/bin/sh\nexit 17\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	p := DefaultProfile()
	p.SSH.Port = 2222
	s := healthySnapshot(PlatformLinux)
	s.SSHService.Running = false
	action := configureSSHAction(p, s)
	for i := range action.Command {
		action.Command[i] = strings.ReplaceAll(action.Command[i], "/etc/ssh/sshd_config", path)
	}
	for i := range action.RollbackCommand {
		action.RollbackCommand[i] = strings.ReplaceAll(action.RollbackCommand[i], "/etc/ssh/sshd_config", path)
	}
	runner := regressionRunner(func(ctx context.Context, cmd []string, out io.Writer) error {
		err := (OSCommandRunner{}).Run(ctx, cmd, out)
		if strings.Contains(cmd[2], "awk ") {
			data, _ := os.ReadFile(path)
			ports := parseConfiguredSSHPorts([]byte(strings.ToLower(string(data))))
			if len(ports) != 1 || ports[0] != 2222 {
				t.Fatalf("old ports survive generated command: %s", data)
			}
		}
		return err
	})
	report, err := (Executor{Runner: runner, AdministratorCheck: func(context.Context, Platform) bool { return true }}).Apply(context.Background(), p, Plan{Platform: PlatformLinux, Actions: []Action{action}}, ApplyOptions{Confirmed: true, JournalDir: t.TempDir()})
	if err == nil || report.Success {
		t.Fatal("validation failure lost")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != string(original) {
		t.Fatalf("preimage not restored: %s %v", data, err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0640 {
		t.Fatal("file mode not restored")
	}
}

func TestGeneratedKeyRollbackWithoutReadyBackupPreservesExistingFile(t *testing.T) {
	dir := t.TempDir()
	sshDir := filepath.Join(dir, ".ssh")
	os.Mkdir(sshDir, 0700)
	path := filepath.Join(sshDir, "authorized_keys")
	os.WriteFile(path, []byte("existing\n"), 0600)
	p := DefaultProfile()
	s := healthySnapshot(PlatformLinux)
	a := configureKeysAction(p, s)
	// Only replace the target lookup with a fixed temporary fixture home. No
	// actual account home, ownership, services or firewall state is touched.
	script := a.RollbackCommand[2]
	start := strings.Index(script, "target_user=")
	end := strings.Index(script, `ssh_dir=`)
	if start < 0 || end < start {
		t.Fatal("target prelude not found")
	}
	script = script[:start] + "target_home=" + shQuote(dir) + "; " + script[end:]
	if out, err := exec.Command("/bin/sh", "-c", script).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "existing\n" {
		t.Fatal("rollback deleted original without a completed preimage")
	}
}
