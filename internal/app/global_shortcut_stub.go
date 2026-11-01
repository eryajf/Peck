//go:build !darwin

package app

import "errors"

func registerGlobalShortcut(func()) error { return errors.New("全局快捷键仅支持 macOS") }
func unregisterGlobalShortcut()           {}
