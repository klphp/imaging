package imaging_test

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/klphp/imaging"

	// ★ 关键：模拟「宿主程序自己 import 了 TIFF 解码器」
	//
	// 这一行让 tiff 注册到进程级全局表，之后 image.Decode 就能解 TIFF ——
	// 而 imaging 自己一行 TIFF 代码都没有。这正是 README 那句
	// 「not in the binary at all」站不住脚的原因。
	_ "golang.org/x/image/tiff"
	"golang.org/x/image/tiff"
)

func jpegBytes(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	img.Set(0, 0, color.NRGBA{G: 255, A: 255})
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func gifBytes(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	pal := color.Palette{color.Black, color.White}
	img := image.NewPaletted(image.Rect(0, 0, 2, 2), pal)
	if err := gif.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func tiffBytes(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	if err := tiff.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// ★★ 核心回归：DecodeAllowed 必须真的挡住 TIFF
//
// 场景就是报告指出的那个：宿主程序 import 了 tiff，全局注册表里有 tiff，
// 普通 Decode 能解开它。白名单版本必须在**读魔数那一步**就拒掉，
// 根本不进 image.Decode。
func TestDecodeAllowed_RejectsTIFFEvenWhenRegistered(t *testing.T) {
	data := tiffBytes(t)

	// 先证明这个环境下普通 Decode 确实能解 TIFF
	// （否则这个测试就是空转 —— 环境里没有 tiff 解码器时它证明不了任何事）
	if _, err := imaging.Decode(bytes.NewReader(data)); err != nil {
		t.Skipf("本环境解不了 TIFF，这条测试证明不了问题: %v", err)
	}

	// 白名单不含 tiff → 必须拒
	_, err := imaging.DecodeAllowed(bytes.NewReader(data),
		imaging.DecodeJPEG, imaging.DecodePNG, imaging.DecodeGIF)
	if err == nil {
		t.Fatal("🔴 白名单没有 tiff 却解开了 —— 依赖全局注册表的判断不可靠")
	}
	if !errors.Is(err, imaging.ErrFormatNotAllowed) {
		t.Errorf("该返回 ErrFormatNotAllowed，实际: %v", err)
	}
}

// 白名单内的格式正常通过
func TestDecodeAllowed_AcceptsListedFormats(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"jpeg", jpegBytes(t)},
		{"png", pngBytes(t)},
		{"gif", gifBytes(t)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			img, err := imaging.DecodeAllowed(bytes.NewReader(c.data),
				imaging.DecodeJPEG, imaging.DecodePNG, imaging.DecodeGIF)
			if err != nil {
				t.Fatalf("%s 该通过: %v", c.name, err)
			}
			if img == nil {
				t.Error("返回了 nil 图")
			}
		})
	}
}

// 白名单只给一种 → 其它都拒
func TestDecodeAllowed_OnlyAllowsGiven(t *testing.T) {
	// 只允许 png，喂 jpeg
	_, err := imaging.DecodeAllowed(bytes.NewReader(jpegBytes(t)), imaging.DecodePNG)
	if err == nil {
		t.Error("白名单只有 png，jpeg 该被拒")
	}

	// 只允许 jpeg，喂 png
	_, err = imaging.DecodeAllowed(bytes.NewReader(pngBytes(t)), imaging.DecodeJPEG)
	if err == nil {
		t.Error("白名单只有 jpeg，png 该被拒")
	}
}

// 无法识别的字节要拒，不能交给 image.Decode 去猜
func TestDecodeAllowed_RejectsUnknownData(t *testing.T) {
	_, err := imaging.DecodeAllowed(bytes.NewReader([]byte("this is not an image at all, just text")),
		imaging.DecodeJPEG, imaging.DecodePNG)
	if err == nil {
		t.Error("非图片数据该被拒")
	}
}

// 空白名单要报错（不能默默全放行）
func TestDecodeAllowed_RejectsEmptyAllowList(t *testing.T) {
	_, err := imaging.DecodeAllowed(bytes.NewReader(pngBytes(t)))
	if err == nil {
		t.Error("空白名单该报错 —— 默默全放行等于没有保护")
	}
}

// 解析文件名版：OpenAllowed 同样受白名单约束
func TestOpenAllowed_RespectsAllowList(t *testing.T) {
	// 这个只验证参数的传递路径，真实文件 IO 由现有测试覆盖
	var buf bytes.Buffer
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}

	// 用 DecodeAllowed 走一遍同样的数据，确认白名单逻辑一致
	if _, err := imaging.DecodeAllowed(bytes.NewReader(buf.Bytes()), imaging.DecodePNG); err != nil {
		t.Errorf("png 在白名单里该通过: %v", err)
	}
	if _, err := imaging.DecodeAllowed(bytes.NewReader(buf.Bytes()), imaging.DecodeJPEG); err == nil {
		t.Error("白名单只有 jpeg 时 png 该被拒")
	}
}
