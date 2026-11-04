package imaging

import (
	"image"
	"image/color"
	"testing"
)

// ★ 复现上游 issue #165：调色板索引越界 panic
//
// 上游 scanner.go:242 是 `c := s.palette[img.Pix[i]]`。
// 如果 Pix 里的值 >= len(palette)，直接 index out of range panic。
//
// 触发条件是 image.Paletted —— 而 **GIF 解码出来就是 Paletted**，
// GIF 又在我们的上传白名单里。所以这个 bug 对我们真实可达，
// 不是"上游只影响 TIFF"。
func TestFork_PaletteIndexOutOfRange_Reproduce(t *testing.T) {
	// 调色板只有 2 个颜色，但像素值是 70
	bad := &image.Paletted{
		Pix:     []uint8{70, 70, 70, 70, 70, 70, 70, 70, 70},
		Stride:  3,
		Rect:    image.Rect(0, 0, 3, 3),
		Palette: color.Palette{color.Black, color.White},
	}

	// 修复前这里会 panic: index out of range [70] with length 2
	out := Resize(bad, 2, 2, Lanczos)
	if out == nil {
		t.Fatal("Resize 返回 nil")
	}

	// 越界的索引应该被当成兜底色（不 panic 就够，颜色不重要）
	t.Logf("越界索引被安全处理，输出 %dx%d", out.Bounds().Dx(), out.Bounds().Dy())
}

// 空调色板也不能 panic
func TestFork_EmptyPalette(t *testing.T) {
	bad := &image.Paletted{
		Pix:     []uint8{0, 0, 0, 0},
		Stride:  2,
		Rect:    image.Rect(0, 0, 2, 2),
		Palette: color.Palette{}, // 完全空
	}

	if out := Resize(bad, 1, 1, Lanczos); out == nil {
		t.Fatal("Resize 返回 nil")
	}
}
