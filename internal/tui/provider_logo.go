package tui

import (
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"strings"
	"sync"
)

// logoRenderRows is the number of terminal rows the rendered logo occupies.
// Each row uses half-block characters to encode two pixel rows.
const logoRenderRows = 8

// logoCache stores previously rendered logo strings keyed by URL.
var logoCache sync.Map

// fetchLogo downloads the image at the given URL and renders it as a small
// block-character art string suitable for display in a terminal. The result
// is cached so repeated calls for the same URL are free.
//
// Returns an empty string if the URL is empty, the download fails, or the
// image cannot be decoded (e.g. SVG files).
func fetchLogo(url string) string {
	if url == "" {
		return ""
	}

	if cached, ok := logoCache.Load(url); ok {
		return cached.(string)
	}

	rendered := downloadAndRender(url, 16, 8)
	logoCache.Store(url, rendered)
	return rendered
}

// downloadAndRender fetches an image from url, resizes it to cols×rows
// (where each row encodes two pixel rows via half-block characters), and
// returns the ANSI-colored block-art string.
func downloadAndRender(url string, cols, rows int) string {
	resp, err := http.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return ""
	}

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		// SVG or unsupported format
		return ""
	}

	resized := resizeNearest(img, cols, rows*2)
	return renderHalfBlocks(resized, cols, rows)
}

// resizeNearest performs nearest-neighbor resize of img to width×height.
func resizeNearest(img image.Image, width, height int) *image.NRGBA {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	dst := image.NewNRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		srcY := bounds.Min.Y + y*srcH/height
		for x := 0; x < width; x++ {
			srcX := bounds.Min.X + x*srcW/width
			dst.Set(x, y, img.At(srcX, srcY))
		}
	}

	return dst
}

// renderHalfBlocks renders a resized image (cols wide, rows*2 tall) as
// terminal block art. Each output row combines two pixel rows using the
// upper-half-block character (▀) with the top pixel as foreground and the
// bottom pixel as background color via ANSI true-color escape sequences.
func renderHalfBlocks(img *image.NRGBA, cols, rows int) string {
	var b strings.Builder

	for row := 0; row < rows; row++ {
		topY := row * 2
		botY := topY + 1

		for col := 0; col < cols; col++ {
			top := img.NRGBAAt(col, topY)
			bot := img.NRGBAAt(col, botY)

			if isTransparent(top) && isTransparent(bot) {
				b.WriteString(" ")
				continue
			}

			if isTransparent(top) {
				// Only bottom pixel visible — use lower half block with fg = bot
				b.WriteString(fmt.Sprintf("\033[38;2;%d;%d;%dm▄\033[0m", bot.R, bot.G, bot.B))
				continue
			}

			if isTransparent(bot) {
				// Only top pixel visible — use upper half block with fg = top
				b.WriteString(fmt.Sprintf("\033[38;2;%d;%d;%dm▀\033[0m", top.R, top.G, top.B))
				continue
			}

			// Both pixels visible — upper half block with fg = top, bg = bot
			b.WriteString(fmt.Sprintf("\033[38;2;%d;%d;%dm\033[48;2;%d;%d;%dm▀\033[0m",
				top.R, top.G, top.B,
				bot.R, bot.G, bot.B))
		}

		if row < rows-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// isTransparent reports whether a color is fully or nearly transparent.
func isTransparent(c color.NRGBA) bool {
	return c.A < 32
}
