# Changelog

All notable changes to this fork. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versioning is
[semantic](https://semver.org/spec/v2.0.0.html).

## [0.1.0] — 2026-11-05

First release of the fork. Based on upstream `disintegration/imaging` v1.6.2
(last upstream commit 2020-12-18).

### Security

- **Removed the TIFF and BMP decoders.** Upstream registered them globally via
  side-effect imports, exposing `image.Decode` to
  [CVE-2023-36308](https://nvd.nist.gov/vuln/detail/CVE-2023-36308)
  (out-of-bounds read in `scanner.go` from a crafted TIFF). The decoders are no
  longer in the binary at all.
- **Fixed a palette-index panic reachable from ordinary GIFs**
  (upstream issue #165). `scanner.go:242` indexed `s.palette[img.Pix[i]]`
  without a bounds check; a malformed `image.Paletted` — and GIF decodes to
  `Paletted` — panicked inside a `parallel()` goroutine, which `recover()`
  cannot catch. Out-of-range indices are now treated as transparent.

### Added

- WebP decoding via `golang.org/x/image/webp` (pure Go, decode only). Requesting
  WebP *output* returns an explicit error rather than a silent fallback.
- Regression tests asserting TIFF/BMP are rejected and JPEG/PNG/GIF still work.
- Tests for the palette panic, including an empty-palette case.

### Changed

- `Resize` and `Fill` no longer upscale. Passing a target larger than the source
  returns the source size — upscaling adds no detail and inflates the file
  (measured: 200×200 → 1600×1600 goes from 7 KB to 107 KB on a synthetic
  gradient).

### Removed

- `adjust.go` — `Adjust*`, `Grayscale`, `Invert` (253 lines)
- `convolution.go` — `Convolve3x3`, `Convolve5x5`, `Blur`, `Sharpen` (148 lines)
- `effects.go` — `Blur`, `Sharpen` wrappers (169 lines)
- `histogram.go` — `Histogram` (52 lines)

  These target image-upload pipelines; adjustment and convolution filters were
  unused and each was another panic surface. Net: 3,758 → 2,190 lines,
  ~40 → 25 exported functions.

### Known limitations

- Animated GIFs decode to the first frame only.
- No HEIC / AVIF (standard library has no decoder).
- No WebP encoding (see README for why).
- `AutoOrientation` reads the file twice via `io.Pipe`.
