//go:build vivahead

package app

// The line-protocol runner is development-only. Release builds never link
// Viva's headless control surface or its offscreen renderer.
import _ "github.com/vivaui/viva/headless/auto"
