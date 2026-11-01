//go:build !darwin

package app

import "github.com/eryajf/Peck/internal/vui"

func loadAppIcon(string) vui.ImageSource { return nil }
