package crontab

import (
	"context"
	"fmt"
)

type FakeClient struct {
	Content     string
	HasCrontab  bool
	ApplyErr    error
	ApplyStderr string
	ReadErr     error
	ApplyCalls  []string
}

func NewFakeClient(content string, hasCrontab bool) *FakeClient {
	return &FakeClient{
		Content:    content,
		HasCrontab: hasCrontab,
	}
}

func (f *FakeClient) Read(_ context.Context) (string, ReadMeta, error) {
	if f.ReadErr != nil {
		return "", ReadMeta{}, f.ReadErr
	}
	if !f.HasCrontab {
		return "", ReadMeta{User: "testuser", IsEmpty: true}, nil
	}
	return f.Content, ReadMeta{User: "testuser", IsEmpty: false}, nil
}

func (f *FakeClient) Apply(_ context.Context, content string) (ApplyResult, error) {
	f.ApplyCalls = append(f.ApplyCalls, content)
	if f.ApplyErr != nil {
		return ApplyResult{Stderr: f.ApplyStderr}, f.ApplyErr
	}
	f.Content = content
	f.HasCrontab = true
	return ApplyResult{}, nil
}

var _ Client = (*FakeClient)(nil)

type FailingClient struct {
	Err error
}

func (f *FailingClient) Read(_ context.Context) (string, ReadMeta, error) {
	return "", ReadMeta{}, f.Err
}

func (f *FailingClient) Apply(_ context.Context, _ string) (ApplyResult, error) {
	return ApplyResult{Stderr: f.Err.Error()}, f.Err
}

var _ Client = (*FailingClient)(nil)

func NewFailingClient(msg string) *FailingClient {
	return &FailingClient{Err: fmt.Errorf("%s", msg)}
}
