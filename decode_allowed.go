package imaging

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"image"
	"io"
	"strings"
)

// DecodeFormat 可解码的图片格式。
//
// 与编码用的 `Format`（int 枚举，只有 JPEG/PNG/GIF）分开：
// **WebP 只能解不能编**，塞进同一个枚举会让人以为能存 WebP。
// 用字符串而不是 iota，是因为这个集合会随解码器增加而变，
// 而且日志/错误信息里直接可读。
type DecodeFormat string

// 本库能解码的格式。
const (
	DecodeJPEG DecodeFormat = "jpeg"
	DecodePNG  DecodeFormat = "png"
	DecodeGIF  DecodeFormat = "gif"
	DecodeWebP DecodeFormat = "webp"
)

// ErrFormatNotAllowed 格式不在白名单里。
var ErrFormatNotAllowed = errors.New("imaging: 图片格式不在白名单内")

// DecodeAllowed 按白名单解码。
//
// # 为什么需要这个，光靠不 import 解码器不够
//
// Go 的 `image.Decode` 用的是**进程级全局注册表**：谁 import 了某个解码器
// （哪怕只为 side effect），它就会被注册进去，之后所有调用方都能解这种格式。
//
// 这不是假设 —— 实测过：宿主程序里写一行
//
//	import _ "golang.org/x/image/tiff"
//
// 之后 `imaging.Decode` 就能解开 TIFF 了，尽管本库自己一行 TIFF 相关代码都没有。
//
// 所以「本库不注册 TIFF/BMP」只保证本库**不是注册的人**，
// 保证不了最终二进制里没有那些解码器。
//
// 要真的限定可解格式，必须显式判格式 —— 这就是这个函数的用途。
// 它先读魔数判断真实格式，不在白名单里就直接拒，根本不进 image.Decode。
//
// # 用法
//
//	// 只接受 JPEG / PNG / WebP
//	img, err := imaging.DecodeAllowed(r,
//	    imaging.DecodeJPEG, imaging.DecodePNG, imaging.DecodeWebP)
//	if errors.Is(err, imaging.ErrFormatNotAllowed) {
//	    return fmt.Errorf("不支持的图片格式")
//	}
//
// 这是上传接口该用的版本。
func DecodeAllowed(r io.Reader, allowed ...DecodeFormat) (image.Image, error) {
	if len(allowed) == 0 {
		return nil, fmt.Errorf("imaging: DecodeAllowed 需要至少一个允许的格式")
	}

	// 先探格式（读魔数），再决定要不要交给 image.Decode
	br := bufio.NewReader(r)
	head, err := br.Peek(32)
	if err != nil && len(head) == 0 {
		return nil, fmt.Errorf("imaging: 读取文件头失败: %w", err)
	}

	actual := detectFormat(head)
	if actual == "" {
		return nil, fmt.Errorf("imaging: 无法识别的图片格式")
	}

	for _, f := range allowed {
		if f == actual {
			img, _, err := image.Decode(br)
			return img, err
		}
	}
	return nil, fmt.Errorf("%w: 检出 %q，允许的是 %s",
		ErrFormatNotAllowed, actual, joinFormats(allowed))
}

// OpenAllowed 按白名单加载文件（DecodeAllowed 的文件版）。
func OpenAllowed(filename string, allowed ...DecodeFormat) (image.Image, error) {
	f, err := fs.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return DecodeAllowed(f, allowed...)
}

// 本库不支持、但能认出来的格式。
//
// 认它们**不是为了解**，是为了报错准确：
// 拒绝一个 TIFF 时告诉用户「tiff 不在白名单里」，
// 比「无法识别的图片格式」有用得多 —— 后者会让人以为是文件坏了。
const (
	formatTIFF DecodeFormat = "tiff"
	formatBMP  DecodeFormat = "bmp"
)

// detectFormat 按魔数判断格式。
//
// 判断原则：**宁可漏判，不可误判**。
// 漏判的后果是拒绝（安全方向），误判的后果是放行（危险方向）。
// 所以每个分支都比对完整魔数，不做「前两字节像就算」这种模糊匹配。
func detectFormat(head []byte) DecodeFormat {
	switch {
	// JPEG: FF D8 FF
	case len(head) >= 3 && head[0] == 0xFF && head[1] == 0xD8 && head[2] == 0xFF:
		return DecodeJPEG

	// PNG: 89 50 4E 47 0D 0A 1A 0A
	case len(head) >= 8 && bytes.Equal(head[:8], []byte("\x89PNG\r\n\x1a\n")):
		return DecodePNG

	// GIF: "GIF87a" / "GIF89a"
	case len(head) >= 6 && (bytes.Equal(head[:6], []byte("GIF87a")) || bytes.Equal(head[:6], []byte("GIF89a"))):
		return DecodeGIF

	// WebP: "RIFF" .... "WEBP"
	case len(head) >= 12 && bytes.Equal(head[:4], []byte("RIFF")) && bytes.Equal(head[8:12], []byte("WEBP")):
		return DecodeWebP

	// ─── 以下不是支持，是为了报错说得准 ───

	// TIFF: "II*\0"（小端）或 "MM\0*"（大端）
	case len(head) >= 4 && (bytes.Equal(head[:4], []byte{0x49, 0x49, 0x2A, 0x00}) ||
		bytes.Equal(head[:4], []byte{0x4D, 0x4D, 0x00, 0x2A})):
		return formatTIFF

	// BMP: "BM"
	case len(head) >= 2 && bytes.Equal(head[:2], []byte("BM")):
		return formatBMP
	}
	return ""
}

func joinFormats(fs []DecodeFormat) string {
	parts := make([]string, len(fs))
	for i, f := range fs {
		parts[i] = string(f)
	}
	return strings.Join(parts, ", ")
}
