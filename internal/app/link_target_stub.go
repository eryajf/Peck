//go:build !darwin

package app

import (
	"context"

	"github.com/eryajf/Peck/internal/core"
)

func findLinkTargets(context.Context, core.LinkRequest) []string { return nil }
