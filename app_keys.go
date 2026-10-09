package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type PublicKeyInfo struct {
	Label          string `json:"label"`
	Path           string `json:"path"`
	PublicKey      string `json:"publicKey"`
	PrivateKeyPath string `json:"privateKeyPath,omitempty"`
	Generated      bool   `json:"generated"`
}

func (a *App) ValidatePublicKey(value string) error {
	return launchpad.ValidatePublicKey(strings.TrimSpace(value))
}

func (a *App) DiscoverPublicKeys() ([]PublicKeyInfo, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	paths, err := filepath.Glob(filepath.Join(home, ".ssh", "*.pub"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	keys := make([]PublicKeyInfo, 0, len(paths))
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil || len(data) > 32*1024 {
			continue
		}
		value := strings.TrimSpace(string(data))
		if launchpad.ValidatePublicKey(value) != nil {
			continue
		}
		keys = append(keys, PublicKeyInfo{
			Label:          filepath.Base(path),
			Path:           path,
			PublicKey:      value,
			PrivateKeyPath: strings.TrimSuffix(path, ".pub"),
		})
	}
	return keys, nil
}

func (a *App) GenerateControllerKey(label string) (PublicKeyInfo, error) {
	sshKeygen, err := exec.LookPath("ssh-keygen")
	if err != nil {
		return PublicKeyInfo{}, errors.New("未找到 ssh-keygen；请先安装 Windows OpenSSH Client")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return PublicKeyInfo{}, err
	}
	directory := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return PublicKeyInfo{}, err
	}
	privatePath := filepath.Join(directory, launchpad.ControllerKeyBaseName)
	publicPath := privatePath + ".pub"
	if _, err := os.Stat(publicPath); errors.Is(err, os.ErrNotExist) {
		if _, privateErr := os.Stat(privatePath); privateErr == nil {
			return PublicKeyInfo{}, errors.New("检测到已有私钥但缺少对应公钥；为避免覆盖，已停止生成。请用 ssh-keygen -y 恢复公钥或选择其他密钥")
		}
		comment := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(label, "\r", " "), "\n", " "))
		if comment == "" {
			comment = "ssh-launchpad-controller"
		}
		command := exec.Command(sshKeygen, "-t", "ed25519", "-f", privatePath, "-N", "", "-C", comment)
		if output, runErr := command.CombinedOutput(); runErr != nil {
			return PublicKeyInfo{}, fmt.Errorf("生成控制电脑密钥失败: %v: %s", runErr, strings.TrimSpace(string(output)))
		}
		_ = os.Chmod(privatePath, 0o600)
		_ = os.Chmod(publicPath, 0o644)
	}
	data, err := os.ReadFile(publicPath)
	if err != nil {
		return PublicKeyInfo{}, err
	}
	value := strings.TrimSpace(string(data))
	if err := launchpad.ValidatePublicKey(value); err != nil {
		return PublicKeyInfo{}, err
	}
	return PublicKeyInfo{
		Label:          filepath.Base(publicPath),
		Path:           publicPath,
		PublicKey:      value,
		PrivateKeyPath: privatePath,
		Generated:      true,
	}, nil
}

func (a *App) ImportPublicKey() (PublicKeyInfo, error) {
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择控制电脑的公钥",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "OpenSSH public key", Pattern: "*.pub;*.txt"},
		},
	})
	if err != nil || path == "" {
		return PublicKeyInfo{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return PublicKeyInfo{}, err
	}
	if len(data) > 32*1024 || strings.Contains(string(data), "PRIVATE KEY") {
		return PublicKeyInfo{}, errors.New("文件不是安全的公钥文件；私钥不会被导入")
	}
	for _, line := range strings.Split(string(data), "\n") {
		value := strings.TrimSpace(line)
		if launchpad.ValidatePublicKey(value) == nil {
			return PublicKeyInfo{Label: filepath.Base(path), Path: path, PublicKey: value}, nil
		}
	}
	return PublicKeyInfo{}, errors.New("文件中没有找到支持的 OpenSSH 公钥")
}

func (a *App) ExportPairingFile(publicKey string) (string, error) {
	publicKey = strings.TrimSpace(publicKey)
	if err := launchpad.ValidatePublicKey(publicKey); err != nil {
		return "", err
	}
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "导出配对公钥",
		DefaultFilename: "ssh-launchpad-controller.pub",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "OpenSSH public key", Pattern: "*.pub"},
		},
	})
	if err != nil || path == "" {
		return "", err
	}
	return path, os.WriteFile(path, []byte(publicKey+"\n"), 0o644)
}
