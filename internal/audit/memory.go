package audit

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/actionlab-ai/aisphere-auth/pkg/aisphereauth"
	"github.com/google/uuid"
)

const defaultMaxEvents = 10000
const defaultListLimit = 100
const maxListLimit = 500

type MemoryOptions struct {
	MaxEvents int
}

type MemoryService struct {
	mu        sync.Mutex
	events    []aisphereauth.AuditEvent
	maxEvents int
	closed    bool
}

func NewMemoryService(opts MemoryOptions) *MemoryService {
	maxEvents := opts.MaxEvents
	if maxEvents <= 0 {
		maxEvents = defaultMaxEvents
	}
	return &MemoryService{maxEvents: maxEvents, events: make([]aisphereauth.AuditEvent, 0)}
}

func (s *MemoryService) Write(ctx context.Context, event aisphereauth.AuditEvent) (*aisphereauth.AuditEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	event = normalizeEvent(event)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, context.Canceled
	}
	if len(s.events) >= s.maxEvents {
		copy(s.events, s.events[1:])
		s.events = s.events[:len(s.events)-1]
	}
	s.events = append(s.events, cloneEvent(event))
	out := cloneEvent(event)
	return &out, nil
}

func (s *MemoryService) List(ctx context.Context, req aisphereauth.AuditListRequest) (*aisphereauth.AuditListResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	limit := req.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	items := make([]aisphereauth.AuditEvent, 0, limit)
	matched := 0
	for i := len(s.events) - 1; i >= 0; i-- {
		event := s.events[i]
		if !matches(event, req) {
			continue
		}
		if matched >= offset && len(items) < limit {
			items = append(items, cloneEvent(event))
		}
		matched++
	}
	return &aisphereauth.AuditListResponse{Items: items, Total: matched, Limit: limit, Offset: offset}, nil
}

func (s *MemoryService) Ping(ctx context.Context) error { return ctx.Err() }

func (s *MemoryService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func normalizeEvent(event aisphereauth.AuditEvent) aisphereauth.AuditEvent {
	event.ID = strings.TrimSpace(event.ID)
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	} else {
		event.CreatedAt = event.CreatedAt.UTC()
	}
	event.TraceID = strings.TrimSpace(event.TraceID)
	event.ActorSubject = strings.TrimSpace(event.ActorSubject)
	event.ActorName = strings.TrimSpace(event.ActorName)
	event.App = strings.TrimSpace(event.App)
	event.ResourceType = strings.TrimSpace(event.ResourceType)
	event.ResourceID = strings.TrimSpace(event.ResourceID)
	event.Action = strings.TrimSpace(event.Action)
	event.Result = strings.TrimSpace(event.Result)
	event.Reason = strings.TrimSpace(event.Reason)
	event.IP = strings.TrimSpace(event.IP)
	event.UserAgent = strings.TrimSpace(event.UserAgent)
	event.RequestPath = strings.TrimSpace(event.RequestPath)
	event.RequestMethod = strings.TrimSpace(event.RequestMethod)
	return event
}

func matches(event aisphereauth.AuditEvent, req aisphereauth.AuditListRequest) bool {
	return match(req.TraceID, event.TraceID) &&
		match(req.ActorSubject, event.ActorSubject) &&
		match(req.App, event.App) &&
		match(req.ResourceType, event.ResourceType) &&
		match(req.ResourceID, event.ResourceID) &&
		match(req.Action, event.Action) &&
		match(req.Result, event.Result)
}

func match(filter string, value string) bool {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return true
	}
	return filter == value
}

func cloneEvent(event aisphereauth.AuditEvent) aisphereauth.AuditEvent {
	if event.Metadata != nil {
		metadata := make(map[string]string, len(event.Metadata))
		for k, v := range event.Metadata {
			metadata[k] = v
		}
		event.Metadata = metadata
	}
	return event
}
