package authgin

import (
	"github.com/actionlab-ai/aisphere-auth/pkg/aisphereauth"
	"github.com/gin-gonic/gin"
)

const principalKey = "aisphere.principal"

func SetPrincipal(c *gin.Context, p *aisphereauth.Principal) {
	c.Set(principalKey, p)
}

func CurrentPrincipal(c *gin.Context) (*aisphereauth.Principal, bool) {
	value, ok := c.Get(principalKey)
	if !ok {
		return nil, false
	}
	p, ok := value.(*aisphereauth.Principal)
	return p, ok
}

func Principal(c *gin.Context) (*aisphereauth.Principal, error) {
	p, ok := CurrentPrincipal(c)
	if !ok || p == nil {
		return nil, aisphereauth.ErrNoPrincipal
	}
	return p, nil
}

// MustPrincipal is kept for backward compatibility. New code should prefer
// Principal or CurrentPrincipal to avoid panic-based control flow.
func MustPrincipal(c *gin.Context) *aisphereauth.Principal {
	p, err := Principal(c)
	if err != nil {
		panic(err)
	}
	return p
}
