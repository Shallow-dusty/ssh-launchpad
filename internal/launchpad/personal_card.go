package launchpad

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	PersonalCardSchemaVersion = 1
	PersonalCardKind          = "ssh-launchpad-personal-card"
	maxPersonalCardBytes      = 1 << 20
)

type PersonalCard struct {
	SchemaVersion  int                   `json:"schemaVersion"`
	Kind           string                `json:"kind"`
	DisplayName    string                `json:"displayName"`
	ControllerName string                `json:"controllerName,omitempty"`
	Note           string                `json:"note,omitempty"`
	SSH            PersonalCardSSH       `json:"ssh"`
	Tailscale      PersonalCardTailscale `json:"tailscale"`
}

type PersonalCardSSH struct {
	Port       int      `json:"port"`
	PublicKeys []string `json:"publicKeys"`
}

type PersonalCardTailscale struct {
	Mode    string `json:"mode"`
	Install bool   `json:"install"`
	AuthKey string `json:"authKey,omitempty"`
}

func NewPersonalCard(profile Profile, displayName, controllerName, note string) PersonalCard {
	return PersonalCard{
		SchemaVersion:  PersonalCardSchemaVersion,
		Kind:           PersonalCardKind,
		DisplayName:    strings.TrimSpace(displayName),
		ControllerName: strings.TrimSpace(controllerName),
		Note:           strings.TrimSpace(note),
		SSH: PersonalCardSSH{
			Port:       profile.SSH.Port,
			PublicKeys: append([]string(nil), profile.SSH.PublicKeys...),
		},
		Tailscale: PersonalCardTailscale{
			Mode:    profile.Transport.Mode,
			Install: profile.Transport.Install,
			AuthKey: strings.TrimSpace(profile.Transport.AuthKey),
		},
	}
}

func (c PersonalCard) Validate() error {
	var errs []error
	if c.SchemaVersion != PersonalCardSchemaVersion {
		errs = append(errs, fmt.Errorf("personal card schemaVersion must be %d", PersonalCardSchemaVersion))
	}
	if c.Kind != PersonalCardKind {
		errs = append(errs, fmt.Errorf("personal card kind must be %q", PersonalCardKind))
	}
	if err := validateCardText("displayName", c.DisplayName, true, 128); err != nil {
		errs = append(errs, err)
	}
	if err := validateCardText("controllerName", c.ControllerName, false, 128); err != nil {
		errs = append(errs, err)
	}
	if err := validateCardText("note", c.Note, false, 1024); err != nil {
		errs = append(errs, err)
	}
	if c.SSH.Port < 1 || c.SSH.Port > 65535 {
		errs = append(errs, errors.New("personal card ssh.port must be between 1 and 65535"))
	}
	if len(c.SSH.PublicKeys) == 0 {
		errs = append(errs, errors.New("personal card requires at least one SSH public key"))
	} else if len(c.SSH.PublicKeys) > 128 {
		errs = append(errs, errors.New("personal card may contain at most 128 SSH public keys"))
	} else {
		for index, key := range c.SSH.PublicKeys {
			if err := ValidatePublicKey(key); err != nil {
				errs = append(errs, fmt.Errorf("personal card ssh.publicKeys[%d]: %w", index, err))
			}
		}
	}
	switch c.Tailscale.Mode {
	case "tailnet", "lan":
	default:
		errs = append(errs, fmt.Errorf("personal card tailscale.mode must be tailnet or lan, got %q", c.Tailscale.Mode))
	}
	profile := c.Profile()
	if err := profile.Validate(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (c PersonalCard) Profile() Profile {
	profile := DefaultProfile()
	profile.Name = strings.TrimSpace(c.DisplayName)
	profile.SSH.Port = c.SSH.Port
	profile.SSH.PublicKeys = append([]string(nil), c.SSH.PublicKeys...)
	profile.Transport.Mode = c.Tailscale.Mode
	profile.Transport.Install = c.Tailscale.Install
	profile.Transport.AuthKey = strings.TrimSpace(c.Tailscale.AuthKey)
	profile.Exposure.Mode = c.Tailscale.Mode
	profile.Labels["cardDisplayName"] = strings.TrimSpace(c.DisplayName)
	if controller := strings.TrimSpace(c.ControllerName); controller != "" {
		profile.Labels["cardControllerName"] = controller
	}
	if note := strings.TrimSpace(c.Note); note != "" {
		profile.Labels["cardNote"] = note
	}
	return profile
}

func LoadPersonalCard(path string) (PersonalCard, error) {
	file, err := os.Open(path)
	if err != nil {
		return PersonalCard{}, fmt.Errorf("read personal card: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxPersonalCardBytes+1))
	if err != nil {
		return PersonalCard{}, fmt.Errorf("read personal card: %w", err)
	}
	if len(data) > maxPersonalCardBytes {
		return PersonalCard{}, errors.New("personal card must not exceed 1 MiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var card PersonalCard
	if err := decoder.Decode(&card); err != nil {
		return PersonalCard{}, fmt.Errorf("parse personal card: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return PersonalCard{}, errors.New("parse personal card: unexpected trailing JSON value")
		}
		return PersonalCard{}, fmt.Errorf("parse personal card: %w", err)
	}
	if err := card.Validate(); err != nil {
		return PersonalCard{}, err
	}
	return card, nil
}

func MarshalPersonalCard(card PersonalCard) ([]byte, error) {
	if err := card.Validate(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(card, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func validateCardText(name, value string, required bool, max int) error {
	value = strings.TrimSpace(value)
	if required && value == "" {
		return fmt.Errorf("personal card %s is required", name)
	}
	if len(value) > max {
		return fmt.Errorf("personal card %s must not exceed %d bytes", name, max)
	}
	if strings.ContainsRune(value, '\x00') {
		return fmt.Errorf("personal card %s contains an invalid character", name)
	}
	return nil
}
