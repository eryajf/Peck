//go:build darwin

package app

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

const (
	carbonCommand = 1 << 8
	carbonShift   = 1 << 9
	carbonKeyP    = 0x23

	carbonKeyboardClass = 0x6b657962 // 'keyb'
	carbonHotKeyPressed = 5
	carbonDirectObject  = 0x2d2d2d2d // '----'
	carbonHotKeyIDType  = 0x686b6964 // 'hkid'
	peckHotKeyID        = 1
	peckHotKeySignature = 0x5065636b // 'Peck'
)

type carbonEventTypeSpec struct {
	class uint32
	kind  uint32
}

type carbonEventHotKeyID struct {
	signature uint32
	id        uint32
}

var carbonHotKey = struct {
	sync.Mutex
	loaded     bool
	callback   func()
	ref        uintptr
	handler    uintptr
	getTarget  func() uintptr
	register   func(uint32, uint32, carbonEventHotKeyID, uintptr, uint32, *uintptr) int32
	unregister func(uintptr) int32
	install    func(uintptr, uintptr, uint32, *carbonEventTypeSpec, uintptr, *uintptr) int32
	getParam   func(uintptr, uint32, uint32, uintptr, uintptr, uintptr, unsafe.Pointer) int32
}{}

var carbonHotKeyCallback = purego.NewCallback(func(_ uintptr, event uintptr, _ uintptr) uintptr {
	carbonHotKey.Lock()
	fn := carbonHotKey.callback
	getParam := carbonHotKey.getParam
	carbonHotKey.Unlock()
	if fn == nil || getParam == nil {
		return 0
	}
	var hotKeyID carbonEventHotKeyID
	if getParam(event, carbonDirectObject, carbonHotKeyIDType, 0, unsafe.Sizeof(hotKeyID), 0, unsafe.Pointer(&hotKeyID)) == 0 && hotKeyID.id == peckHotKeyID {
		fn()
	}
	return 0
})

func registerGlobalShortcut(fn func()) error {
	carbonHotKey.Lock()
	defer carbonHotKey.Unlock()
	if fn == nil {
		return fmt.Errorf("快捷键回调为空")
	}
	if carbonHotKey.loaded {
		return fmt.Errorf("Peck 全局快捷键已注册")
	}
	lib, err := purego.Dlopen("/System/Library/Frameworks/Carbon.framework/Carbon", purego.RTLD_GLOBAL|purego.RTLD_NOW)
	if err != nil {
		return fmt.Errorf("加载 macOS 快捷键服务：%w", err)
	}
	purego.RegisterLibFunc(&carbonHotKey.getTarget, lib, "GetApplicationEventTarget")
	purego.RegisterLibFunc(&carbonHotKey.register, lib, "RegisterEventHotKey")
	purego.RegisterLibFunc(&carbonHotKey.unregister, lib, "UnregisterEventHotKey")
	purego.RegisterLibFunc(&carbonHotKey.install, lib, "InstallEventHandler")
	purego.RegisterLibFunc(&carbonHotKey.getParam, lib, "GetEventParameter")
	spec := carbonEventTypeSpec{class: carbonKeyboardClass, kind: carbonHotKeyPressed}
	if status := carbonHotKey.install(carbonHotKey.getTarget(), carbonHotKeyCallback, 1, &spec, 0, &carbonHotKey.handler); status != 0 {
		return fmt.Errorf("安装 macOS 快捷键处理器失败（%d）", status)
	}
	if status := carbonHotKey.register(carbonKeyP, carbonCommand|carbonShift, carbonEventHotKeyID{signature: peckHotKeySignature, id: peckHotKeyID}, carbonHotKey.getTarget(), 0, &carbonHotKey.ref); status != 0 {
		return fmt.Errorf("注册 ⇧⌘P 失败（可能已被其他应用占用，错误码 %d）", status)
	}
	carbonHotKey.callback = fn
	carbonHotKey.loaded = true
	return nil
}

func unregisterGlobalShortcut() {
	carbonHotKey.Lock()
	defer carbonHotKey.Unlock()
	if carbonHotKey.loaded && carbonHotKey.unregister != nil {
		carbonHotKey.unregister(carbonHotKey.ref)
	}
	carbonHotKey.callback = nil
	carbonHotKey.ref = 0
	carbonHotKey.loaded = false
}
