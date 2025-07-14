package tmuxmcp

type TmuxTool struct {
}

type SessionTool struct {
	TmuxTool
	Prefix  string `json:"prefix" description:"Session name prefix (auto-detected from git repo if not provided)"`
	Session string `json:"session" description:"Specific session name (overrides prefix)"`
}
