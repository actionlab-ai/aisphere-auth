package audit

import "context"

type Event struct {
	TraceID string
	Subject string
	Object  string
	Action  string
	Allow   bool
	Reason  string
}

type Service interface {
	Write(ctx context.Context, event Event) error
}
