package launchpad

import "time"

const (
	SchemaVersion = 1
)

var Version = "0.1.0"

type Stage string

const (
	StageCheck    Stage = "check"
	StagePlan     Stage = "plan"
	StageApply    Stage = "apply"
	StageVerify   Stage = "verify"
	StageRollback Stage = "rollback"
)

type Platform string

const (
	PlatformAuto    Platform = "auto"
	PlatformWindows Platform = "windows"
	PlatformLinux   Platform = "linux"
	PlatformMacOS   Platform = "macos"
	PlatformWSL     Platform = "wsl"
)

type Risk string

const (
	RiskLow      Risk = "low"
	RiskMedium   Risk = "medium"
	RiskHigh     Risk = "high"
	RiskCritical Risk = "critical"
)

type Profile struct {
	SchemaVersion int               `json:"schemaVersion" yaml:"schemaVersion"`
	Name          string            `json:"name" yaml:"name"`
	Target        TargetProfile     `json:"target" yaml:"target"`
	SSH           SSHProfile        `json:"ssh" yaml:"ssh"`
	Transport     TransportProfile  `json:"transport" yaml:"transport"`
	Exposure      ExposureProfile   `json:"exposure" yaml:"exposure"`
	Download      DownloadProfile   `json:"download" yaml:"download"`
	Safety        SafetyProfile     `json:"safety" yaml:"safety"`
	Advanced      AdvancedProfile   `json:"advanced" yaml:"advanced"`
	Labels        map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

type TargetProfile struct {
	Platform Platform `json:"platform" yaml:"platform"`
	WSL      bool     `json:"wsl,omitempty" yaml:"wsl,omitempty"`
}

type SSHProfile struct {
	Enabled                bool     `json:"enabled" yaml:"enabled"`
	Port                   int      `json:"port" yaml:"port"`
	PublicKeys             []string `json:"publicKeys,omitempty" yaml:"publicKeys,omitempty"`
	PasswordAuthentication bool     `json:"passwordAuthentication" yaml:"passwordAuthentication"`
}

type TransportProfile struct {
	Mode    string `json:"mode" yaml:"mode"`
	Install bool   `json:"install" yaml:"install"`
}

type ExposureProfile struct {
	Mode        string   `json:"mode" yaml:"mode"`
	CustomCIDRs []string `json:"customCidrs,omitempty" yaml:"customCidrs,omitempty"`
}

type DownloadProfile struct {
	Strategy      string `json:"strategy" yaml:"strategy"`
	MirrorBaseURL string `json:"mirrorBaseUrl,omitempty" yaml:"mirrorBaseUrl,omitempty"`
	ProxyURL      string `json:"proxyUrl,omitempty" yaml:"proxyUrl,omitempty"`
	OfflineBundle string `json:"offlineBundle,omitempty" yaml:"offlineBundle,omitempty"`
	CacheDir      string `json:"cacheDir,omitempty" yaml:"cacheDir,omitempty"`
	Retries       int    `json:"retries" yaml:"retries"`
}

type SafetyProfile struct {
	ConfirmHighRisk      bool `json:"confirmHighRisk" yaml:"confirmHighRisk"`
	PreventSelfCut       bool `json:"preventSelfCut" yaml:"preventSelfCut"`
	ScheduledDelaySecond int  `json:"scheduledDelaySeconds" yaml:"scheduledDelaySeconds"`
	AutoRollback         bool `json:"autoRollback" yaml:"autoRollback"`
}

type AdvancedProfile struct {
	WindowsSSHService string `json:"windowsSshService,omitempty" yaml:"windowsSshService,omitempty"`
	LinuxSSHService   string `json:"linuxSshService,omitempty" yaml:"linuxSshService,omitempty"`
	MacOSSSHLabel     string `json:"macosSshLabel,omitempty" yaml:"macosSshLabel,omitempty"`
	StateDir          string `json:"stateDir,omitempty" yaml:"stateDir,omitempty"`
}

type Snapshot struct {
	Timestamp        time.Time      `json:"timestamp"`
	Platform         Platform       `json:"platform"`
	Arch             string         `json:"arch"`
	Hostname         string         `json:"hostname"`
	IsAdministrator  bool           `json:"isAdministrator"`
	SessionTransport string         `json:"sessionTransport"`
	PackageManager   string         `json:"packageManager,omitempty"`
	SSHClient        Capability     `json:"sshClient"`
	SSHServer        Capability     `json:"sshServer"`
	SSHService       ServiceState   `json:"sshService"`
	SSHPort          int            `json:"sshPort,omitempty"`
	SSHConfigValid   bool           `json:"sshConfigValid"`
	Firewall         FirewallState  `json:"firewall"`
	Tailscale        TransportState `json:"tailscale"`
	Network          NetworkState   `json:"network"`
	PlatformDetails  map[string]any `json:"platformDetails,omitempty"`
	Warnings         []string       `json:"warnings,omitempty"`
	ProbeErrors      []string       `json:"probeErrors,omitempty"`
}

type Capability struct {
	Installed bool   `json:"installed"`
	Path      string `json:"path,omitempty"`
	Version   string `json:"version,omitempty"`
}

type ServiceState struct {
	Name        string `json:"name,omitempty"`
	Installed   bool   `json:"installed"`
	Running     bool   `json:"running"`
	StartPolicy string `json:"startPolicy,omitempty"`
}

type FirewallState struct {
	Provider string   `json:"provider,omitempty"`
	Ports    []int    `json:"ports,omitempty"`
	Scopes   []string `json:"scopes,omitempty"`
}

type TransportState struct {
	Installed bool   `json:"installed"`
	Online    bool   `json:"online"`
	IP        string `json:"ip,omitempty"`
	State     string `json:"state,omitempty"`
}

type NetworkState struct {
	GitHubDNS    bool `json:"githubDns"`
	TailscaleDNS bool `json:"tailscaleDns"`
	ProxySet     bool `json:"proxySet"`
}

type Action struct {
	ID                string            `json:"id"`
	Operation         string            `json:"operation"`
	Layer             string            `json:"layer"`
	Risk              Risk              `json:"risk"`
	Summary           string            `json:"summary"`
	Reason            string            `json:"reason"`
	Mutating          bool              `json:"mutating"`
	RequiresElevation bool              `json:"requiresElevation"`
	SelfCutRisk       bool              `json:"selfCutRisk"`
	Reversible        bool              `json:"reversible"`
	Command           []string          `json:"command,omitempty"`
	RollbackCommand   []string          `json:"rollbackCommand,omitempty"`
	Params            map[string]string `json:"params,omitempty"`
}

type Plan struct {
	Timestamp       time.Time `json:"timestamp"`
	ProfileName     string    `json:"profileName"`
	Platform        Platform  `json:"platform"`
	ReadOnly        bool      `json:"readOnly"`
	NoChanges       bool      `json:"noChanges"`
	HighestRisk     Risk      `json:"highestRisk"`
	SelfCutDetected bool      `json:"selfCutDetected"`
	Actions         []Action  `json:"actions"`
	Warnings        []string  `json:"warnings,omitempty"`
}

type Event struct {
	Timestamp time.Time `json:"timestamp"`
	Stage     Stage     `json:"stage"`
	ActionID  string    `json:"actionId,omitempty"`
	Kind      string    `json:"kind"`
	Message   string    `json:"message"`
}

type ActionResult struct {
	ActionID string    `json:"actionId"`
	Status   string    `json:"status"`
	Started  time.Time `json:"started"`
	Finished time.Time `json:"finished"`
	Output   string    `json:"output,omitempty"`
	Error    string    `json:"error,omitempty"`
}

type Report struct {
	SchemaVersion int            `json:"schemaVersion"`
	Version       string         `json:"version"`
	ID            string         `json:"id"`
	Stage         Stage          `json:"stage"`
	Started       time.Time      `json:"started"`
	Finished      time.Time      `json:"finished"`
	Success       bool           `json:"success"`
	ExitCode      int            `json:"exitCode"`
	ProfileName   string         `json:"profileName"`
	Snapshot      *Snapshot      `json:"snapshot,omitempty"`
	Plan          *Plan          `json:"plan,omitempty"`
	Results       []ActionResult `json:"results,omitempty"`
	Events        []Event        `json:"events,omitempty"`
	JournalPath   string         `json:"journalPath,omitempty"`
	Warnings      []string       `json:"warnings,omitempty"`
	Error         string         `json:"error,omitempty"`
}

type Journal struct {
	SchemaVersion int            `json:"schemaVersion"`
	ID            string         `json:"id"`
	Created       time.Time      `json:"created"`
	ProfileName   string         `json:"profileName"`
	Status        string         `json:"status"`
	Actions       []Action       `json:"actions"`
	Results       []ActionResult `json:"results"`
}

type ApplyOptions struct {
	Confirmed      bool
	AllowSelfCut   bool
	ScheduleRisky  bool
	AutoRollback   bool
	JournalDir     string
	ExternalVerify string
}

type EventSink func(Event)

const (
	ExitOK                   = 0
	ExitInvalidProfile       = 2
	ExitVerificationFailed   = 3
	ExitNeedsElevation       = 4
	ExitConfirmationRequired = 5
	ExitSelfCutBlocked       = 6
	ExitPartialFailure       = 7
	ExitDownloadFailure      = 8
	ExitUnsupported          = 9
)
