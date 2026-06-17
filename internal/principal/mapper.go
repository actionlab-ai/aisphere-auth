package principal

import (
	"strings"
	"time"

	"github.com/actionlab-ai/aisphere-auth/internal/casdoor"
)

func FromCasdoorUser(app string, fallbackOwner string, u *casdoor.UserInfo) *Principal {
	if u == nil {
		return &Principal{App: app, AuthProvider: "casdoor", AuthTimeUnix: time.Now().Unix()}
	}
	owner := firstNonEmpty(u.Owner, fallbackOwner)
	name := u.Name
	if name == "" && strings.Contains(u.ID, "/") {
		parts := strings.SplitN(u.ID, "/", 2)
		owner = firstNonEmpty(owner, parts[0])
		name = parts[1]
	}
	casdoorSubject := ""
	if owner != "" && name != "" {
		casdoorSubject = owner + "/" + name
	}
	subjectID := u.ID
	if subjectID == "" {
		subjectID = casdoorSubject
	}
	if subjectID != "" && !strings.Contains(subjectID, ":") {
		subjectID = "human:" + subjectID
	}
	return &Principal{SubjectID: subjectID, CasdoorSubject: casdoorSubject, Username: name, DisplayName: u.DisplayName, Email: u.Email, Organization: owner, Roles: u.Roles, Groups: u.Groups, App: app, AuthProvider: "casdoor", AuthTimeUnix: time.Now().Unix()}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
