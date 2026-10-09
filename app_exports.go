package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"gopkg.in/yaml.v3"
)

func (a *App) ExportReport(report launchpad.Report) (string, error) {
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Export SSH Launchpad report",
		DefaultFilename: report.ID + ".report.json",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "JSON report", Pattern: "*.json"},
		},
	})
	if err != nil || path == "" {
		return "", err
	}
	data, err := json.MarshalIndent(launchpad.RedactReport(report), "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	return path, writePrivateFile(path, append(data, '\n'))
}

func (a *App) ImportProfile() (launchpad.Profile, error) {
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "导入 SSH Launchpad 配置",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "YAML / JSON profile", Pattern: "*.yaml;*.yml;*.json"},
		},
	})
	if err != nil || path == "" {
		return launchpad.Profile{}, err
	}
	return launchpad.LoadProfile(path)
}

func (a *App) ExportProfile(profile launchpad.Profile) (string, error) {
	if err := profile.Validate(); err != nil {
		return "", err
	}
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "导出 SSH Launchpad 配置",
		DefaultFilename: profile.Name + ".ssh-launchpad.yaml",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "YAML profile", Pattern: "*.yaml"},
		},
	})
	if err != nil || path == "" {
		return "", err
	}
	data, err := marshalExportProfile(profile)
	if err != nil {
		return "", err
	}
	return path, writePrivateFile(path, data)
}

// marshalExportProfile strips secrets before a profile leaves the app: a
// YAML export is meant to be shareable, so the Tailscale auth key never
// travels with it.
func marshalExportProfile(profile launchpad.Profile) ([]byte, error) {
	profile.Transport.AuthKey = ""
	return yaml.Marshal(profile)
}

func (a *App) ImportPersonalCard() (launchpad.PersonalCard, error) {
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "导入 SSH Launchpad 装机卡",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "SSH Launchpad personal card", Pattern: "*.sshlaunchpad-card;*.json"},
		},
	})
	if err != nil || path == "" {
		return launchpad.PersonalCard{}, err
	}
	return launchpad.LoadPersonalCard(path)
}

func (a *App) ExportPersonalCard(card launchpad.PersonalCard) (string, error) {
	data, err := launchpad.MarshalPersonalCard(card)
	if err != nil {
		return "", err
	}
	filename := safeCardFilename(card.DisplayName) + ".sshlaunchpad-card"
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "导出 SSH Launchpad 装机卡",
		DefaultFilename: filename,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "SSH Launchpad personal card", Pattern: "*.sshlaunchpad-card"},
		},
	})
	if err != nil || path == "" {
		return "", err
	}
	if !strings.HasSuffix(strings.ToLower(path), ".sshlaunchpad-card") {
		path += ".sshlaunchpad-card"
	}
	return path, writePrivateFile(path, data)
}

func safeCardFilename(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Map(func(r rune) rune {
		switch r {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			return '-'
		default:
			if r < 32 {
				return -1
			}
			return r
		}
	}, value)
	value = strings.Trim(value, ". ")
	if value == "" {
		return "ssh-launchpad-personal"
	}
	return value
}

func writePrivateFile(path string, data []byte) error {
	// Stage privately before replacement: never truncate or follow an existing
	// broad-permission export, and leave its original contents on failure.
	file, err := os.CreateTemp(filepath.Dir(path), ".ssh-launchpad-export-*")
	if err != nil {
		return err
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
