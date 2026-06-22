package iam

import (
	"context"
	"testing"

	"github.com/actionlab-ai/aisphere-auth/pkg/aisphereauth"
)

func TestResourceGrantUserViewerAllowsReadOnly(t *testing.T) {
	svc := NewMemoryService()
	_, err := svc.SaveResourceGrant(context.Background(), aisphereauth.ResourceGrant{App: "aihub", ResourceType: "skill", ResourceID: "search", SubjectType: "user", SubjectID: "aisphere/test1", Role: "viewer"})
	if err != nil {
		t.Fatalf("save grant: %v", err)
	}
	read, _ := svc.CheckResourceGrant(context.Background(), aisphereauth.ResourceGrantCheckRequest{Subject: "aisphere/test1", App: "aihub", Object: "aihub:skill:search", Action: "read"})
	if read == nil || !read.Allow {
		t.Fatalf("expected read allow, got %#v", read)
	}
	write, _ := svc.CheckResourceGrant(context.Background(), aisphereauth.ResourceGrantCheckRequest{Subject: "aisphere/test1", App: "aihub", Object: "aihub:skill:search", Action: "update"})
	if write == nil || write.Allow {
		t.Fatalf("expected update deny, got %#v", write)
	}
}

func TestResourceGrantGroupHierarchyAllowsChildGroup(t *testing.T) {
	svc := NewMemoryService()
	_, err := svc.SaveResourceGrant(context.Background(), aisphereauth.ResourceGrant{App: "aihub", ResourceType: "skillset", ResourceID: "ops", SubjectType: "group", SubjectID: "org-a/platform", Role: "editor"})
	if err != nil {
		t.Fatalf("save grant: %v", err)
	}
	decision, _ := svc.CheckResourceGrant(context.Background(), aisphereauth.ResourceGrantCheckRequest{Principal: &aisphereauth.Principal{SubjectID: "human:dev1", Groups: []string{"org-a/platform/devops"}}, App: "aihub", Object: "aihub:skillset:ops", Action: "update"})
	if decision == nil || !decision.Allow {
		t.Fatalf("expected child group allow, got %#v", decision)
	}
}

func TestResourceGrantPublicViewer(t *testing.T) {
	svc := NewMemoryService()
	_, err := svc.SaveResourceGrant(context.Background(), aisphereauth.ResourceGrant{App: "aihub", ResourceType: "skill", ResourceID: "public-skill", SubjectType: "public", Role: "viewer"})
	if err != nil {
		t.Fatalf("save grant: %v", err)
	}
	decision, _ := svc.CheckResourceGrant(context.Background(), aisphereauth.ResourceGrantCheckRequest{App: "aihub", Object: "aihub:skill:public-skill", Action: "read"})
	if decision == nil || !decision.Allow {
		t.Fatalf("expected public read allow, got %#v", decision)
	}
}
