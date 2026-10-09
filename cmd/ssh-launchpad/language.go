package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type language string

const (
	langAuto language = "auto"
	langZH   language = "zh-CN"
	langEN   language = "en"
)

var currentLanguage = langEN

func resolveLanguage(requested language) language {
	if requested == langAuto {
		if env := os.Getenv("SSH_LAUNCHPAD_LANG"); env != "" {
			requested = language(env)
		}
	}
	if requested == langAuto {
		if saved := savedLanguage(); saved != langAuto {
			requested = saved
		}
	}
	if requested == langAuto {
		if runtime.GOOS != "windows" {
			locale := strings.ToLower(os.Getenv("LC_ALL") + " " + os.Getenv("LANG"))
			if locale != " " && !strings.Contains(locale, "utf-8") && !strings.Contains(locale, "utf8") {
				return langEN
			}
			if strings.Contains(locale, "zh") {
				return langZH
			}
		}
		if strings.HasPrefix(strings.ToLower(os.Getenv("LANG")), "zh") {
			return langZH
		}
		return systemLanguage()
	}
	return requested
}

func persistLanguage(value language) error {
	directory, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	path := filepath.Join(directory, "SSH Launchpad", "language")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(value+"\n"), 0o600)
}

func savedLanguage() language {
	directory, err := os.UserConfigDir()
	if err != nil {
		return langAuto
	}
	data, err := os.ReadFile(filepath.Join(directory, "SSH Launchpad", "language"))
	if err != nil {
		return langAuto
	}
	value := language(strings.TrimSpace(string(data)))
	if value == langZH || value == langEN {
		return value
	}
	return langAuto
}

func glyph(ascii, unicode string) string {
	if runtime.GOOS != "windows" {
		locale := strings.ToLower(os.Getenv("LC_ALL") + " " + os.Getenv("LANG"))
		if locale != " " && !strings.Contains(locale, "utf-8") && !strings.Contains(locale, "utf8") {
			return ascii
		}
	}
	return unicode
}
