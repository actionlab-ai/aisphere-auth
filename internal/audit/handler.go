package audit

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/actionlab-ai/aisphere-auth/internal/httpx"
	"github.com/actionlab-ai/aisphere-auth/pkg/aisphereauth"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Write(c *gin.Context) {
	var event aisphereauth.AuditEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		httpx.RespondError(c, http.StatusBadRequest, "invalid_audit_event", "审计事件格式不正确")
		return
	}
	if err := validateEvent(event); err != nil {
		httpx.RespondError(c, http.StatusBadRequest, "invalid_audit_event", err.Error())
		return
	}
	if event.TraceID == "" {
		event.TraceID = httpx.RequestID(c)
	}
	stored, err := h.svc.Write(c.Request.Context(), event)
	if err != nil {
		httpx.RespondError(c, http.StatusInternalServerError, "audit_write_failed", "写入审计事件失败")
		return
	}
	c.JSON(http.StatusCreated, stored)
}

func (h *Handler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	resp, err := h.svc.List(c.Request.Context(), aisphereauth.AuditListRequest{
		TraceID:      c.Query("traceId"),
		ActorSubject: c.Query("actorSubject"),
		App:          c.Query("app"),
		ResourceType: c.Query("resourceType"),
		ResourceID:   c.Query("resourceId"),
		Action:       c.Query("action"),
		Result:       c.Query("result"),
		Limit:        limit,
		Offset:       offset,
	})
	if err != nil {
		httpx.RespondError(c, http.StatusInternalServerError, "audit_list_failed", "查询审计事件失败")
		return
	}
	c.JSON(http.StatusOK, resp)
}

func validateEvent(event aisphereauth.AuditEvent) error {
	switch {
	case strings.TrimSpace(event.ActorSubject) == "":
		return errText("actorSubject 不能为空")
	case strings.TrimSpace(event.ResourceType) == "":
		return errText("resourceType 不能为空")
	case strings.TrimSpace(event.Action) == "":
		return errText("action 不能为空")
	case strings.TrimSpace(event.Result) == "":
		return errText("result 不能为空")
	default:
		return nil
	}
}

type errText string

func (e errText) Error() string { return string(e) }
