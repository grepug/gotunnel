package protocol

type AuthRequest struct {
	AgentID string `json:"agent_id"`
	Token   string `json:"token"`
}

type RegisterRequest struct {
	AgentID string   `json:"agent_id"`
	Targets []string `json:"targets"`
}

type OpenRequest struct {
	Target string `json:"target"`
}

type OpenResult struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}
