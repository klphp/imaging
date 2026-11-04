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

	// ★ 不 panic 不够 —— 要断言**越界像素实际输出什么**。
	//
	// 只验证「没崩」的话，实现改成「越界时返回随机内存颜色」也能通过，
	// 而那在业务上就是个坏像素（可能整块发黑或发绿）。
	// 约定是：越界索引按**全透明**处理（宁可不显示，也不显示错误的颜色）。
	for y := 0; y < out.Bounds().Dy(); y++ {
		for x := 0; x < out.Bounds().Dx(); x++ {
			c := out.NRGBAAt(x, y)
			if c.A != 0 {
				t.Errorf("越界像素 (%d,%d) 该是透明，实际 %v —— 越界索引不该被映射成任何实色", x, y, c)
			}
		}
	}
}

// 空调色板也不能 panic
func TestFork_EmptyPalette(t *testing.T) {
	bad := &image.Paletted{
		Pix:     []uint8{0, 0, 0, 0},
		Stride:  2,
		Rect:    image.Rect(0, 0, 2, 2),
		Palette: color.Palette{}, // 完全空
	}

	out := Resize(bad, 1, 1, Lanczos)
	if out == nil {
		t.Fatal("Resize 返回 nil")
	}
	// 空调色板 → 所有像素都越界 → 全部透明
	if c := out.NRGBAAt(0, 0); c.A != 0 {
		t.Errorf("空调色板的输出该全透明，实际 %v", c)
	}
}

// 部分越界：调色板内的索引正常取色，越界的透明
func TestFork_PalettePartialOutOfRange(t *testing.T) {
	// 2×1 的图：第一个像素索引 0（合法，黑色），第二个索引 99（越界）
	img := &image.Paletted{
		Pix:     []uint8{0, 99},
		Stride:  2,
		Rect:    image.Rect(0, 0, 2, 1),
		Palette: color.Palette{color.Black, color.White},
	}

	out := Resize(img, 2, 1, Lanczos)
	if out == nil {
		t.Fatal("Resize 返回 nil")
	}

	left := out.NRGBAAt(0, 0)
	if left.A == 0 {
		t.Errorf("合法索引 0（黑色）该有颜色，实际透明 %v", left)
	}
	right := out.NRGBAAt(1, 0)
	if right.A != 0 {
		t.Errorf("越界索引 99 该透明，实际 %v", right)
	}
}
