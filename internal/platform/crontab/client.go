package crontab

import "context"

type ReadMeta struct {
	User    string
	IsEmpty bool
}

type ApplyResult struct {
	Stderr string
}

type Client interface {
	Read(ctx context.Context) (text string, meta ReadMeta, err error)
	Apply(ctx context.Context, content string) (ApplyResult, error)
}
