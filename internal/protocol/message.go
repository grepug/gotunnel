package protocol

type AuthRequest struct {
	Token string `json:"token"`
}

type RegisterRequest struct {
	Targets []string `json:"targets"`
}

type OpenRequest struct {
	Target string `json:"target"`
}

type OpenResult struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}
