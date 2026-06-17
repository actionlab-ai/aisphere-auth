package principal

// Principal is the normalized AI Sphere identity context shared by all platform services.
type Principal struct {
	SubjectID      string   `json:"subjectId"`
	CasdoorSubject string   `json:"casdoorSubject"`
	Username       string   `json:"username"`
	DisplayName    string   `json:"displayName,omitempty"`
	Email          string   `json:"email,omitempty"`
	Organization   string   `json:"organization"`
	Roles          []string `json:"roles,omitempty"`
	Groups         []string `json:"groups,omitempty"`

	App           string `json:"app,omitempty"`
	SessionID     string `json:"sessionId,omitempty"`
	TokenID       string `json:"tokenId,omitempty"`
	AuthProvider  string `json:"authProvider"`
	AuthTimeUnix  int64  `json:"authTimeUnix,omitempty"`
	ExpiresAtUnix int64  `json:"expiresAtUnix,omitempty"`
}

func (p *Principal) EffectiveSubject() string {
	if p == nil {
		return ""
	}
	if p.CasdoorSubject != "" {
		return p.CasdoorSubject
	}
	return p.SubjectID
}
