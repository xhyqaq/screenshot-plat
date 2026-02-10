package capture

import (
	"bytes"
	"fmt"
	"github.com/kbinani/screenshot"
	"image/png"
)

// PrimaryPNG 捕获主显示器并返回 PNG 字节
func PrimaryPNG() ([]byte, error) {
	// 只捕获主显示器 0；后续可扩展多显示器或区域
	bounds := screenshot.GetDisplayBounds(0)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return nil, fmt.Errorf("capture screen: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return buf.Bytes(), nil
}
