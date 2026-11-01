//go:build !vivahead

package app

import "errors"

// renderPreview is kept out of ordinary application binaries. The Viva
// headless renderer is an opt-in development tool; build with -tags vivahead
// when exporting the five preview states.
func renderPreview(string) error {
	return errors.New("预览导出需要使用 -tags vivahead 构建")
}
