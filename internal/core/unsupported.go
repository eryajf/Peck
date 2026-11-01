//go:build !darwin

package core

import (
	"context"
	"errors"
)

type snapshot struct{}

func Inspect(context.Context, string) (*Inspection, error) {
	return nil, errors.New("Peck 的应用处理仅支持 macOS")
}

func Process(context.Context, *Inspection, Options, func(Progress)) Result {
	return Result{Signature: "未请求", Quarantine: "未处理", Launch: "未请求", Error: errors.New("Peck 的应用处理仅支持 macOS")}
}

func Open(context.Context, *Result) error {
	return errors.New("Peck 的应用处理仅支持 macOS")
}
