package imaging

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"
)

// makePNG 造一张有内容的 PNG。
func makePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.NRGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// ★ 核心断言：TIFF / BMP 解码器已从二进制里移除
//
// 这是 fork 的主要目的之一 —— 上游 imaging import 了
// golang.org/x/image/tiff 和 bmp，通过副作用注册到 image 包，
// 于是 imaging.Decode → image.Decode 能解 TIFF。
//
// 上游的 CVE-2023-36308（scanner.go 的 scan 越界，攻击者提交特制 TIFF）
// 就靠这条路利用。我们把解码器删掉 —— 攻击面从「配置守住」
// 变成「代码里不存在」。
func TestFork_RejectsTIFFAndBMP(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"TIFF 小端", []byte{0x49, 0x49, 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00}},
		{"TIFF 大端", []byte{0x4D, 0x4D, 0x00, 0x2A, 0x00, 0x00, 0x00, 0x08}},
		{"BMP", []byte{0x42, 0x4D, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
	}

	for _, c := range cases {
		_, cfgFormat, err := image.DecodeConfig(bytes.NewReader(c.data))
		if err == nil {
			t.Errorf("%s：DecodeConfig 认出了格式 %q —— 解码器还在二进制里，"+
				"CVE-2023-36308 的利用路径仍然存在", c.name, cfgFormat)
		}

		if _, err := Decode(bytes.NewReader(c.data)); err == nil {
			t.Errorf("%s：Decode 没报错 —— 解码器还在", c.name)
		}
	}
}

// JPEG / PNG / GIF 仍然正常（只删了 TIFF/BMP）
func TestFork_StillSupportsJPEGPNGGIF(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))

	var jbuf, gbuf bytes.Buffer
	if err := jpeg.Encode(&jbuf, img, nil); err != nil {
		t.Fatal(err)
	}
	if err := gif.Encode(&gbuf, img, nil); err != nil {
		t.Fatal(err)
	}
	pngData := makePNG(t, 20, 20)

	cases := []struct {
		name      string
		data      []byte
		wantFmtNm string
	}{
		{"JPEG", jbuf.Bytes(), "jpeg"},
		{"PNG", pngData, "png"},
		{"GIF", gbuf.Bytes(), "gif"},
	}

	for _, c := range cases {
		got, _, err := image.DecodeConfig(bytes.NewReader(c.data))
		if err != nil {
			t.Errorf("%s：该认出来，实际报错 %v", c.name, err)
			continue
		}
		if got.Width != 20 || got.Height != 20 {
			t.Errorf("%s：尺寸应为 20×20，实际 %dx%d", c.name, got.Width, got.Height)
		}

		decoded, err := Decode(bytes.NewReader(c.data))
		if err != nil {
			t.Errorf("%s：Decode 失败 %v", c.name, err)
			continue
		}
		if decoded.Bounds().Dx() != 20 {
			t.Errorf("%s：解码后宽应为 20，实际 %d", c.name, decoded.Bounds().Dx())
		}
	}
}

// 核心变换仍然工作 —— 裁剪掉 convolution/adjust/histogram 不能碰到这些
func TestFork_CoreTransformsWork(t *testing.T) {
	src := makePNG(t, 400, 300)

	t.Run("Resize", func(t *testing.T) {
		img, err := Decode(bytes.NewReader(src))
		if err != nil {
			t.Fatal(err)
		}
		out := Resize(img, 200, 0, Lanczos)
		if out.Bounds().Dx() != 200 {
			t.Errorf("宽应为 200，实际 %d", out.Bounds().Dx())
		}
		if out.Bounds().Dy() != 150 {
			t.Errorf("等比高应为 150，实际 %d", out.Bounds().Dy())
		}
	})

	t.Run("Fill", func(t *testing.T) {
		img, _ := Decode(bytes.NewReader(src))
		out := Fill(img, 100, 100, Center, Lanczos)
		if b := out.Bounds(); b.Dx() != 100 || b.Dy() != 100 {
			t.Errorf("应为 100×100，实际 %dx%d", b.Dx(), b.Dy())
		}
	})

	t.Run("Fit", func(t *testing.T) {
		img, _ := Decode(bytes.NewReader(src))
		out := Fit(img, 100, 100, Lanczos)
		if b := out.Bounds(); b.Dx() > 100 || b.Dy() > 100 {
			t.Errorf("该放进 100×100，实际 %dx%d", b.Dx(), b.Dy())
		}
	})

	t.Run("Crop", func(t *testing.T) {
		img, _ := Decode(bytes.NewReader(src))
		out := Crop(img, image.Rect(10, 10, 110, 60))
		if b := out.Bounds(); b.Dx() != 100 || b.Dy() != 50 {
			t.Errorf("应为 100×50，实际 %dx%d", b.Dx(), b.Dy())
		}
	})

	t.Run("Overlay", func(t *testing.T) {
		base, _ := Decode(bytes.NewReader(src))
		mark := image.NewNRGBA(image.Rect(0, 0, 50, 50))
		for i := range mark.Pix {
			mark.Pix[i] = 255
		}
		out := Overlay(base, mark, image.Pt(10, 10), 1.0)
		if out == nil {
			t.Fatal("Overlay 返回 nil")
		}
	})

	t.Run("五档滤镜", func(t *testing.T) {
		img, _ := Decode(bytes.NewReader(src))
		for name, f := range map[string]ResampleFilter{
			"NearestNeighbor": NearestNeighbor,
			"Box":             Box,
			"Linear":          Linear,
			"CatmullRom":      CatmullRom,
			"Lanczos":         Lanczos,
		} {
			out := Resize(img, 50, 0, f)
			if out == nil || out.Bounds().Dx() != 50 {
				t.Errorf("%s 滤镜结果异常", name)
			}
		}
	})
}

// 透明通道必须保留（fork 后仍然）
func TestFork_PreservesAlpha(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	for x := 0; x < 100; x++ {
		for y := 0; y < 100; y++ {
			if x > 20 && x < 80 && y > 20 && y < 80 {
				img.Set(x, y, color.NRGBA{R: 255, A: 255})
			}
		}
	}

	out := Resize(img, 50, 50, Lanczos)
	_, _, _, a := out.At(1, 1).RGBA()
	if a>>8 != 0 {
		t.Errorf("边角该是透明的，alpha 实际 %d", a>>8)
	}
}
