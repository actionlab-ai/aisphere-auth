package iamhttp

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/actionlab-ai/aisphere-auth/internal/httpx"
	"github.com/actionlab-ai/aisphere-auth/internal/iam"
	"github.com/actionlab-ai/aisphere-auth/pkg/aisphereauth"
	"github.com/gin-gonic/gin"
)

type Handler struct{ svc iam.Service }

func NewHandler(svc iam.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) CreateResourceGrant(c *gin.Context) {
	var grant aisphereauth.ResourceGrant
	if err := c.ShouldBindJSON(&grant); err != nil {
		httpx.RespondError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if grant.App == "" || grant.ResourceType == "" || grant.ResourceID == "" || grant.SubjectType == "" || grant.Role == "" {
		httpx.RespondError(c, http.StatusBadRequest, "invalid_request", "app, resourceType, resourceId, subjectType and role are required")
		return
	}
	out, err := h.svc.SaveResourceGrant(c.Request.Context(), grant)
	if err != nil {
		httpx.RespondError(c, http.StatusInternalServerError, "iam_error", err.Error())
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) ListResourceGrants(c *gin.Context) {
	q := aisphereauth.ResourceGrantQuery{
		App:          c.Query("app"),
		OrgID:        c.Query("orgId"),
		ProjectID:    c.Query("projectId"),
		ResourceType: c.Query("resourceType"),
		ResourceID:   c.Query("resourceId"),
		Object:       c.Query("object"),
		SubjectType:  c.Query("subjectType"),
		SubjectID:    c.Query("subjectId"),
		Limit:        atoi(c.Query("limit")),
		Offset:       atoi(c.Query("offset")),
	}
	out, err := h.svc.ListResourceGrants(c.Request.Context(), q)
	if err != nil {
		httpx.RespondError(c, http.StatusInternalServerError, "iam_error", err.Error())
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) DeleteResourceGrant(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		httpx.RespondError(c, http.StatusBadRequest, "invalid_request", "grant id is required")
		return
	}
	if err := h.svc.DeleteResourceGrant(c.Request.Context(), id); err != nil {
		httpx.RespondError(c, http.StatusInternalServerError, "iam_error", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true, "id": id})
}

func (h *Handler) CheckResourceGrant(c *gin.Context) {
	var req aisphereauth.ResourceGrantCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.RespondError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if req.TraceID == "" {
		req.TraceID = httpx.RequestID(c)
	}
	out, err := h.svc.CheckResourceGrant(c.Request.Context(), req)
	if err != nil {
		httpx.RespondError(c, http.StatusInternalServerError, "iam_error", err.Error())
		return
	}
	c.JSON(http.StatusOK, out)
}

func atoi(s string) int {
	i, _ := strconv.Atoi(strings.TrimSpace(s))
	return i
}
