package crontab

import (
	"context"
	"testing"
)

func TestFakeClient_ReadEmpty(t *testing.T) {
	fc := NewFakeClient("", false)
	text, meta, err := fc.Read(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !meta.IsEmpty {
		t.Fatal("expected IsEmpty=true for no crontab")
	}
	if text != "" {
		t.Fatalf("expected empty text, got %q", text)
	}
}

func TestFakeClient_ReadWithContent(t *testing.T) {
	content := "0 3 * * * /bin/backup\n"
	fc := NewFakeClient(content, true)
	text, meta, err := fc.Read(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.IsEmpty {
		t.Fatal("expected IsEmpty=false")
	}
	if text != content {
		t.Fatalf("expected %q, got %q", content, text)
	}
}

func TestFakeClient_ApplyUpdatesContent(t *testing.T) {
	fc := NewFakeClient("old\n", true)
	newContent := "0 4 * * * /bin/new\n"
	_, err := fc.Apply(context.Background(), newContent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text, _, _ := fc.Read(context.Background())
	if text != newContent {
		t.Fatalf("expected content to be updated, got %q", text)
	}
	if len(fc.ApplyCalls) != 1 {
		t.Fatalf("expected 1 apply call, got %d", len(fc.ApplyCalls))
	}
}

func TestFakeClient_ApplyError(t *testing.T) {
	fc := NewFakeClient("old\n", true)
	fc.ApplyErr = NewFailingClient("bad syntax").Err
	fc.ApplyStderr = "errors in crontab file"
	result, err := fc.Apply(context.Background(), "bad\n")
	if err == nil {
		t.Fatal("expected error")
	}
	if result.Stderr != "errors in crontab file" {
		t.Fatalf("expected stderr, got %q", result.Stderr)
	}
	text, _, _ := fc.Read(context.Background())
	if text != "old\n" {
		t.Fatal("content should not change on apply error")
	}
}

func TestFailingClient_ReadError(t *testing.T) {
	fc := NewFailingClient("permission denied")
	_, _, err := fc.Read(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "permission denied" {
		t.Fatalf("unexpected error: %v", err)
	}
}
