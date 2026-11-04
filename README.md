# imaging

[![Test](https://github.com/klphp/imaging/actions/workflows/test.yml/badge.svg)](https://github.com/klphp/imaging/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/klphp/imaging.svg)](https://pkg.go.dev/github.com/klphp/imaging)
[![Go Report Card](https://goreportcard.com/badge/github.com/klphp/imaging)](https://goreportcard.com/report/github.com/klphp/imaging)

A maintained fork of [`github.com/disintegration/imaging`](https://github.com/disintegration/imaging) —
image resizing, cropping and compositing for Go. Pure Go, no CGo.

```go
src, _ := imaging.Open("photo.jpg")
thumb := imaging.Fill(src, 200, 200, imaging.Center, imaging.Lanczos)
imaging.Save(thumb, "thumb.jpg", imaging.JPEGQuality(85))
```

---

## Why this fork

Upstream has had **no commits since December 2020** and still carries 36 open
issues, including an unfixed CVE. The resampling algorithms themselves are fine
— Lanczos filtering doesn't age — but nothing gets fixed, and no fork has picked
up the work (the 478 forks are all one-off copies with zero stars).

This fork does three things.

### 1. Removes the TIFF and BMP decoders

Upstream `io.go` imports `golang.org/x/image/tiff` and `golang.org/x/image/bmp`
for their side effects, which registers them globally with `image.Decode`.
That is the entry point for [CVE-2023-36308](https://nvd.nist.gov/vuln/detail/CVE-2023-36308)
— an out-of-bounds read in `scanner.go` triggered by a crafted TIFF.

We don't decode TIFF or BMP, so the imports are gone. **This library no longer
registers those decoders** — the CVE entry point is removed from our own code
path.

> **⚠️ What this does NOT guarantee**
>
> Go's `image.Decode` uses a **process-wide registry**. Any program (or any
> dependency it imports) that does `import _ "golang.org/x/image/tiff"` will
> register the TIFF decoder globally, and `imaging.Decode` — which delegates to
> `image.Decode` — will happily decode TIFF again.
>
> This is not hypothetical. Verified: a host program with that single blank
> import can decode a TIFF through `imaging.Decode` even though this library
> contains no TIFF code at all.
>
> So the accurate claim is: *this library is no longer the one registering them*.
> Not: *the final binary cannot decode them*.
>
> **If you need a hard format limit — e.g. an upload endpoint — use
> `DecodeAllowed`** (below). It sniffs the magic bytes and rejects anything
> outside your allow-list *before* reaching `image.Decode`, so the global
> registry never gets a say.

### Enforcing an allow-list

```go
img, err := imaging.DecodeAllowed(r,
    imaging.DecodeJPEG, imaging.DecodePNG, imaging.DecodeWebP)
if errors.Is(err, imaging.ErrFormatNotAllowed) {
    return fmt.Errorf("unsupported image format")
}
```

`DecodeAllowed` reads the header and compares it against the allow-list first.
Unknown magic → rejected with `ErrFormatNotAllowed`. TIFF/BMP are recognised by
magic bytes specifically so the error message says `"tiff"` instead of the
useless "unrecognised format".

### 2. Fixes the palette-index panic (upstream issue #165)

Upstream `scanner.go:242`:

```go
c := s.palette[img.Pix[i]]   // panics when the pixel value >= len(palette)
```

A malformed `image.Paletted` — and **GIF decodes to `Paletted`** — takes the
whole process down:

```
panic: runtime error: index out of range [70] with length 2
    imaging.(*scanner).scan          scanner.go:242
    imaging.resizeHorizontal.func1   resize.go:114
```

The panic happens inside a goroutine spawned by `parallel()`, so a `recover()`
in your HTTP middleware will not save you. This is reachable from ordinary
user-uploaded GIFs, which makes it more dangerous in practice than the CVE.

Out-of-range indices are now treated as transparent pixels.

### 3. Adds WebP decoding

`golang.org/x/image/webp` is registered, so `imaging.Open` and `image.Decode`
accept WebP input — the format most "save image as" downloads and phone
screenshots arrive in.

**There is no WebP encoder** and this fork does not add one. `x/image/webp`
ships only `decode.go`; every mature encoder (`bimg`, `govips`, `imgproxy`,
`imaginary`) needs CGo + libwebp, which defeats the point of a pure-Go single
binary. The pure-Go candidates are not production-ready yet
(`gen2brain/webp` is 63 stars of WASM-transpiled libwebp; `gen2brain/vpx` is a
13-star project created three months ago).

Asking for WebP output gives you a clear error, never a silent fallback:

```
unsupported output format "webp" (only jpeg / png / gif)
```

If you use this in an upload pipeline, map WebP input to PNG or JPEG depending
on whether the image has transparency — `image.Image.Opaque()` tells you without
scanning pixels.

---

## Install

```bash
go get github.com/klphp/imaging
```

Requires Go 1.21+.

---

## Differences from upstream

| | upstream | this fork |
|---|---|---|
| TIFF / BMP decode | ✅ (CVE path) | ❌ removed (not registered by this lib — use `DecodeAllowed` for a hard limit) |
| WebP decode | ❌ | ✅ |
| WebP encode | ❌ | ❌ |
| Palette index panic (#165) | 💥 | fixed |
| `Grayscale` / `Invert` / `Adjust*` | ✅ | ❌ removed |
| `Blur` / `Sharpen` / `Convolve*` | ✅ | ❌ removed |
| `Histogram` | ✅ | ❌ removed |
| Lines of code | 3,758 | 2,190 |
| Exported functions | ~40 | 25 |

**The removals are deliberate.** This fork targets image-upload pipelines:
resize, crop, thumbnail, watermark. Adjustment and convolution filters were dead
weight there, and each one is another code path that can panic on hostile input.

Need them? Use upstream, or open an issue and make the case.

---

## API

### Resizing

```go
imaging.Resize(img, 800, 0, imaging.Lanczos)                  // width 800, height auto
imaging.Fit(img, 800, 600, imaging.Lanczos)                   // fit inside box, no distortion
imaging.Fill(img, 200, 200, imaging.Center, imaging.Lanczos)  // fill then center-crop
```

`Fill` always returns a square — handy for avatar and product-grid thumbnails.

**Neither `Resize` nor `Fill` upscales.** Pass a target larger than the source
and you get the source size back. Upscaling cannot add detail; it only produces
a blurry file that is much larger (measured on a synthetic gradient: a 200×200
source asked for 1600×1600 goes from 7 KB to 107 KB, with zero added detail).

### Transforms

```go
imaging.Crop(img, image.Rect(10, 10, 110, 110))
imaging.Rotate90(img)     // also Rotate180, Rotate270
imaging.FlipH(img)        // also FlipV
imaging.Transpose(img)    // also Transverse
```

### Compositing

```go
imaging.Overlay(background, watermark, image.Pt(10, 10), 0.5)  // 50% opacity
imaging.OverlayCenter(background, watermark, 0.7)
imaging.Paste(background, img, image.Pt(0, 0))
```

### Resampling filters

`imaging.NearestNeighbor`, `imaging.Box`, `imaging.Linear`,
`imaging.CatmullRom`, `imaging.Lanczos`

`NearestNeighbor` is fastest and lowest quality (correct for pixel art);
`Lanczos` is slowest and best (correct for photos).

### I/O

```go
img, err := imaging.Open("in.png")                                // ignores EXIF rotation
img, err := imaging.Open("in.png", imaging.AutoOrientation(true)) // applies EXIF orientation
err := imaging.Save(img, "out.jpg", imaging.JPEGQuality(85))
```

---

## Transparency

Everything returns `*image.NRGBA`, so alpha survives `Resize`, `Crop`, `Fill`
and `Overlay`.

**Watch out for JPEG.** Go's `jpeg.Encode` ignores the alpha channel completely
and encodes the RGB components as-is. Transparent pixels in a freshly created
`image.NRGBA` are `(0,0,0,0)`, so encoding a transparent PNG to JPEG without
flattening gives you a **black background, not white**:

```go
// Wrong — black background where the image was transparent
jpeg.Encode(w, transparentImg, nil)

// Right — flatten onto a background colour first
dst := image.NewRGBA(transparentImg.Bounds())
draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
draw.Draw(dst, dst.Bounds(), transparentImg, transparentImg.Bounds().Min, draw.Over)
jpeg.Encode(w, dst, nil)
```

This bites people constantly and the symptom looks like a bug in the resize
code. If your pipeline converts transparent images to JPEG, flatten explicitly.

---

## Memory limits

`imaging.Open` will happily try to decode a 50,000 × 50,000 image and consume
about 10 GB of RAM doing it. If you accept images from users, **check the
dimensions before decoding**:

```go
cfg, _, err := image.DecodeConfig(r)   // reads the header only, allocates nothing
if err != nil {
    return err
}
if cfg.Width*cfg.Height > 50_000_000 {
    return errors.New("image dimensions out of range")
}
```

This guard needs no credentials to exploit, which makes it the single most
important check in a public upload path.

A note on error messages: a polyglot file (`GIF89a;<php ...>`) parses as a valid
GIF header, so `DecodeConfig` returns nonsense dimensions like 15419×28735.
Don't phrase the error as "image too large" — say "invalid or unsupported
dimensions", because it may not be a real image at all.

---

## Known limitations

- **Animated GIFs are not supported** — only the first frame is decoded.
- **No HEIC / AVIF** — the standard library doesn't decode them.
- **`AutoOrientation` reads the file twice** (it uses an `io.Pipe` to scan EXIF
  while decoding). On large files that is a measurable cost; if you don't need
  EXIF rotation — e.g. you already stripped metadata — skip the option.

---

## Contributing

Issues and PRs are welcome, especially:

- Bug reports with a failing test case (malformed input is the highest value)
- Performance work in `scanner.go` — it's the inner loop of every resample
- Benchmarks on architectures beyond amd64 / arm64

Please don't send back the features that were removed (adjustments, convolution
filters). That decision is deliberate; see the comparison table.

If upstream ever resumes maintenance and fixes these issues, the fixes will be
contributed back rather than maintained here forever.

---

## License

MIT, same as upstream. Original work copyright © 2012 Grigory Dryapak.
See [LICENSE](LICENSE).

---

## 中文说明

这个仓库是 `disintegration/imaging` 的维护分支，最初为 Go Web 框架
[ginbase](https://github.com/klphp) 的文件上传模块而 fork。三处改动：移除
TIFF/BMP 解码器（消掉 CVE 入口）、修复 GIF 触发的 panic、增加 WebP 解码。

如果你只是想找上游的替代品，删除掉不需要的滤镜部分是这个 fork 的取舍 ——
需要亮度/对比度/模糊/锐化请直接用上游。
