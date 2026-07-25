package elevation

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"

	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
)

const (
	SchemaVersion            = 1
	WindowsCancelledExitCode = 1223
	maxProtocolFileBytes     = 1 << 20
)

var ErrCancelled = errors.New("elevation request cancelled")

type Request struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Profile       launchpad.Profile      `json:"profile"`
	Options       launchpad.ApplyOptions `json:"options"`
	ResponsePath  string                 `json:"responsePath"`
	EventsPath    string                 `json:"eventsPath,omitempty"`
	Language      string                 `json:"language,omitempty"`
}

type Response struct {
	Report launchpad.Report `json:"report"`
	Error  string           `json:"error,omitempty"`
}

func NewRequest(profile launchpad.Profile, options launchpad.ApplyOptions, responsePath, eventsPath, language string) Request {
	return Request{
		SchemaVersion: SchemaVersion,
		Profile:       profile,
		Options:       options,
		ResponsePath:  responsePath,
		EventsPath:    eventsPath,
		Language:      language,
	}
}

func PrecreateFile(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("elevation protocol path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	return file.Close()
}

func WriteRequest(path string, request Request) (string, error) {
	if err := validateRequest(request); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return Digest(data), nil
}

func VerifyRequest(path, expectedDigest string) (Request, error) {
	data, err := readBoundedRegularFile(path)
	if err != nil {
		return Request{}, err
	}
	if !strings.EqualFold(Digest(data), strings.TrimSpace(expectedDigest)) {
		return Request{}, errors.New("elevated request integrity check failed")
	}
	var request Request
	if err := json.Unmarshal(data, &request); err != nil {
		return Request{}, err
	}
	if err := validateRequest(request); err != nil {
		return Request{}, err
	}
	if err := validateProtocolPaths(path, request); err != nil {
		return Request{}, err
	}
	return request, nil
}

func validateRequest(request Request) error {
	if request.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported elevation request schema %d", request.SchemaVersion)
	}
	if !request.Options.Confirmed {
		return errors.New("elevated helper accepts only explicitly confirmed Apply requests")
	}
	if strings.TrimSpace(request.ResponsePath) == "" {
		return errors.New("elevation response path is required")
	}
	if err := request.Profile.Validate(); err != nil {
		return err
	}
	return nil
}

func validateProtocolPaths(requestPath string, request Request) error {
	directory := filepath.Clean(filepath.Dir(requestPath))
	if filepath.Clean(filepath.Dir(request.ResponsePath)) != directory ||
		filepath.Base(request.ResponsePath) != "response.json" {
		return errors.New("elevation response path must be the precreated response.json beside the request")
	}
	if request.EventsPath != "" &&
		(filepath.Clean(filepath.Dir(request.EventsPath)) != directory ||
			filepath.Base(request.EventsPath) != "events.jsonl") {
		return errors.New("elevation event path must be the precreated events.jsonl beside the request")
	}
	return nil
}

func Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func WriteResponse(path string, response Response) error {
	data, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return err
	}
	file, err := openExistingWriteFile(path, false)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func ReadResponse(path string) (Response, error) {
	data, err := readBoundedRegularFile(path)
	if err != nil {
		return Response{}, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return Response{}, errors.New("elevated helper returned an empty response")
	}
	var response Response
	if err := json.Unmarshal(data, &response); err != nil {
		return Response{}, err
	}
	return response, nil
}

func OpenEventFile(path string) (*os.File, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("elevation event path is required")
	}
	return openExistingWriteFile(path, true)
}

func readBoundedRegularFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("elevation protocol path must be a regular file")
	}
	if info.Size() > maxProtocolFileBytes {
		return nil, errors.New("elevation protocol file exceeds the size limit")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, maxProtocolFileBytes+1))
}

func ParseArguments(args []string) (requestPath, digest string, err error) {
	values := map[string]string{}
	for index := 0; index < len(args); index++ {
		if !strings.HasPrefix(args[index], "--") || index+1 >= len(args) {
			return "", "", errors.New("invalid elevated helper arguments")
		}
		values[strings.TrimPrefix(args[index], "--")] = args[index+1]
		index++
	}
	requestPath = strings.TrimSpace(values["request"])
	digest = strings.TrimSpace(values["sha256"])
	if requestPath == "" || digest == "" {
		return "", "", errors.New("elevated helper requires --request and --sha256")
	}
	return requestPath, digest, nil
}

func WindowsStartProcessScript(executable string, arguments []string) string {
	quoted := make([]string, len(arguments))
	for index, argument := range arguments {
		quoted[index] = QuoteWindowsArgument(argument)
	}
	argumentLine := strings.Join(quoted, " ")
	return fmt.Sprintf(
		"$ErrorActionPreference='Stop'; try {"+
			"$p=Start-Process -FilePath %s -ArgumentList %s -Verb RunAs -Wait -PassThru; exit $p.ExitCode "+
			"} catch { $e=$_.Exception; while ($null -ne $e) { "+
			"if (($e -is [System.ComponentModel.Win32Exception]) -and ($e.NativeErrorCode -eq %d)) { exit %d }; "+
			"$e=$e.InnerException }; [Console]::Error.WriteLine($_.Exception.Message); exit 1 }",
		powerShellSingleQuote(executable),
		powerShellSingleQuote(argumentLine),
		WindowsCancelledExitCode,
		WindowsCancelledExitCode,
	)
}

func QuoteWindowsArgument(value string) string {
	if value == "" {
		return `""`
	}
	if !strings.ContainsAny(value, " \t\n\v\"") {
		return value
	}
	var quoted strings.Builder
	quoted.WriteByte('"')
	backslashes := 0
	for _, character := range value {
		if character == '\\' {
			backslashes++
			continue
		}
		if character == '"' {
			quoted.WriteString(strings.Repeat(`\`, backslashes*2+1))
			quoted.WriteRune(character)
			backslashes = 0
			continue
		}
		quoted.WriteString(strings.Repeat(`\`, backslashes))
		backslashes = 0
		quoted.WriteRune(character)
	}
	quoted.WriteString(strings.Repeat(`\`, backslashes*2))
	quoted.WriteByte('"')
	return quoted.String()
}

func powerShellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func UTF16LEBase64(value string) string {
	var encoded bytes.Buffer
	for _, unit := range utf16.Encode([]rune(value)) {
		_ = binary.Write(&encoded, binary.LittleEndian, unit)
	}
	return base64.StdEncoding.EncodeToString(encoded.Bytes())
}
