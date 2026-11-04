package imaging_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/klphp/imaging"
)

// mkSolid 造一张纯色图，便于逐像素比对。
func mkSolid(w, h int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// ★★ 回归：Resize 永不放大
//
// README 第 130 行写着「Neither Resize nor Fill upscales」，但实现里
// 没有任何尺寸检查 —— 200×200 调 Resize(1600,1600) 会真的插值放大到
// 1600×1600。文档是契约，按它修实现。
func TestResize_NeverUpscales(t *testing.T) {
	src := mkSolid(200, 200, color.NRGBA{R: 10, G: 20, B: 30, A: 255})

	cases := []struct {
		name    string
		w, h    int
		wantW   int
		wantH   int
	}{
		{"两个维度都更大", 1600, 1600, 200, 200},
		{"只给宽（等比）", 1600, 0, 200, 200},
		{"只给高（等比）", 0, 1600, 200, 200},
		{"略大一点也不放", 201, 201, 200, 200},
		{"恰好相同", 200, 200, 200, 200},
		{"只有一个维度更大（宽）", 400, 100, 200, 100},
		{"只有一个维度更大（高）", 100, 400, 100, 200},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := imaging.Resize(src, c.w, c.h, imaging.Lanczos)
			got := out.Bounds().Size()
			if got.X != c.wantW || got.Y != c.wantH {
				t.Errorf("Resize(%d,%d) 该得到 %d×%d，实际 %d×%d",
					c.w, c.h, c.wantW, c.wantH, got.X, got.Y)
			}
		})
	}
}

// ★ 不放大时返回的必须是**原图内容**，不是一张被重采样过的糊图
//
// 只断言尺寸不够：如果实现是「缩到 1×1 再拉回来」，尺寸也可能对，
// 但像素全毁了。这里逐像素比对。
func TestResize_NoUpscalePreservesPixels(t *testing.T) {
	// 用四色块，任何重采样都会改变颜色
	src := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	src.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	src.SetNRGBA(3, 0, color.NRGBA{G: 255, A: 255})
	src.SetNRGBA(0, 3, color.NRGBA{B: 255, A: 255})
	src.SetNRGBA(3, 3, color.NRGBA{R: 255, G: 255, A: 255})

	out := imaging.Resize(src, 400, 400, imaging.Lanczos)

	if out.Bounds().Dx() != 4 || out.Bounds().Dy() != 4 {
		t.Fatalf("该保持 4×4，实际 %v", out.Bounds().Size())
	}

	for _, p := range []image.Point{{0, 0}, {3, 0}, {0, 3}, {3, 3}} {
		want := src.NRGBAAt(p.X, p.Y)
		got := out.NRGBAAt(p.X, p.Y)
		if got != want {
			t.Errorf("像素 %v 被改动了: want %v got %v —— 不该走重采样", p, want, got)
		}
	}
}

// ★★ 回归：Fill 也不放大
func TestFill_NeverUpscales(t *testing.T) {
	src := mkSolid(200, 150, color.NRGBA{R: 10, G: 20, B: 30, A: 255})

	cases := []struct {
		name         string
		w, h         int
		wantW, wantH int
	}{
		{"两个维度都更大", 1600, 1600, 200, 150},
		{"只宽更大", 400, 150, 200, 150},
		{"只高更大", 200, 400, 200, 150},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := imaging.Fill(src, c.w, c.h, imaging.Center, imaging.Lanczos)
			got := out.Bounds().Size()
			if got.X != c.wantW || got.Y != c.wantH {
				t.Errorf("Fill(%d,%d) 该得到 %d×%d，实际 %d×%d",
					c.w, c.h, c.wantW, c.wantH, got.X, got.Y)
			}
		})
	}
}

// 缩小必须真的缩小（别为了防放大把正常缩放也拦了）
func TestResize_StillDownscales(t *testing.T) {
	src := mkSolid(800, 600, color.NRGBA{R: 10, G: 20, B: 30, A: 255})

	cases := []struct{ w, h, wantW, wantH int }{
		{400, 300, 400, 300},
		{100, 100, 100, 100},
		{0, 300, 400, 300}, // 等比
		{400, 0, 400, 300}, // 等比
		{1, 1, 1, 1},
	}
	for _, c := range cases {
		out := imaging.Resize(src, c.w, c.h, imaging.Lanczos)
		got := out.Bounds().Size()
		if got.X != c.wantW || got.Y != c.wantH {
			t.Errorf("Resize(%d,%d) 该得到 %d×%d，实际 %d×%d",
				c.w, c.h, c.wantW, c.wantH, got.X, got.Y)
		}
	}

	// Fill 缩小
	out := imaging.Fill(src, 200, 200, imaging.Center, imaging.Lanczos)
	if out.Bounds().Dx() != 200 || out.Bounds().Dy() != 200 {
		t.Errorf("Fill 缩小该得到 200×200，实际 %v", out.Bounds().Size())
	}
}

// 缩放后的像素值要合理（用纯色图验证没有插值出奇怪颜色）
func TestResize_DownscaleKeepsColor(t *testing.T) {
	want := color.NRGBA{R: 10, G: 20, B: 30, A: 255}
	src := mkSolid(400, 400, want)

	out := imaging.Resize(src, 100, 100, imaging.Lanczos)
	// 纯色图缩放后中心必须还是这个颜色（边缘可能有滤波振铃，取中心）
	got := out.NRGBAAt(50, 50)
	// 允许 1 的误差（浮点舍入）
	if abs(int(got.R)-int(want.R)) > 1 || abs(int(got.G)-int(want.G)) > 1 ||
		abs(int(got.B)-int(want.B)) > 1 || abs(int(got.A)-int(want.A)) > 1 {
		t.Errorf("纯色图缩放后中心颜色变了: want %v got %v", want, got)
	}
}

// 无效尺寸的容错（0×0 / 负数返回空图，不能 panic）
func TestResize_InvalidSizes(t *testing.T) {
	src := mkSolid(10, 10, color.NRGBA{A: 255})
	for _, c := range []struct{ w, h int }{{0, 0}, {-1, 10}, {10, -1}, {-5, -5}} {
		out := imaging.Resize(src, c.w, c.h, imaging.Lanczos)
		if out == nil {
			t.Errorf("Resize(%d,%d) 不该返回 nil", c.w, c.h)
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
