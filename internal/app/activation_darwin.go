//go:build darwin && !vivahead

package app

import "github.com/ebitengine/purego/objc"

// setAccessoryActivationPolicy keeps Peck a menu-bar application after Viva
// has initialized NSApplication. Viva deliberately starts native applications
// as regular apps so ordinary windows behave well; Peck's menu-bar workflow
// keeps it out of the Dock with the accessory policy.
func setAccessoryActivationPolicy() {
	application := objc.ID(objc.GetClass("NSApplication")).Send(objc.RegisterName("sharedApplication"))
	application.Send(objc.RegisterName("setActivationPolicy:"), uintptr(1))
}
