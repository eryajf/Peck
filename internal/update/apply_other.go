//go:build !darwin

package update

import "errors"

func StartApply(string) error                     { return errors.New("in-place updates are supported on macOS only") }
func RunApply([]string) error                     { return errors.New("in-place updates are supported on macOS only") }
func ConfirmStartup(string, string, string) error { return nil }
