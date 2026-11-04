// Package imaging provides basic image processing functions for Go:
// resizing, cropping, rotating, flipping and compositing.
//
// All functions accept any image.Image and return a new *image.NRGBA
// (32-bit RGBA with non-premultiplied alpha), so transparency survives
// every operation.
//
// # A maintained fork
//
// This is a fork of github.com/disintegration/imaging, which has been
// unmaintained since December 2020. The changes are deliberate and narrow:
//
//   - The TIFF and BMP decoders are removed. Upstream registered them
//     globally through side-effect imports, which put CVE-2023-36308
//     (a crafted-TIFF out-of-bounds read in scanner.go) on the decode path.
//   - A palette-index panic reachable from ordinary GIFs is fixed
//     (upstream issue #165). It fired inside a goroutine, so recover()
//     could not catch it.
//   - WebP decoding is supported via golang.org/x/image/webp. There is no
//     WebP *encoder* — asking for that returns an error instead of silently
//     falling back.
//   - Adjustment and convolution filters were removed. They were unused for
//     the upload-pipeline use case and each was another panic surface.
//
// Resize and Fill no longer upscale: passing a target larger than the source
// returns the source size, because interpolation cannot add detail.
//
// See the README for the full comparison with upstream.
package imaging
