package screen

import (
	"image"
	"image/jpeg"
	"io"

	"github.com/kbinani/screenshot"
)

// Display descreve um monitor ativo.
type Display struct {
	Index   int  `json:"index"`
	Primary bool `json:"primary"`
	Width   int  `json:"width"`
	Height  int  `json:"height"`
}

// Count retorna quantos monitores estão ativos.
func Count() int { return screenshot.NumActiveDisplays() }

// Displays lista os monitores ativos (índice 0 é o principal).
func Displays() []Display {
	n := screenshot.NumActiveDisplays()
	out := make([]Display, 0, n)
	for i := 0; i < n; i++ {
		b := screenshot.GetDisplayBounds(i)
		out = append(out, Display{Index: i, Primary: i == 0, Width: b.Dx(), Height: b.Dy()})
	}
	return out
}

func boundsAt(index int) image.Rectangle {
	n := screenshot.NumActiveDisplays()
	if n <= 0 {
		return image.Rect(0, 0, 1, 1)
	}
	if index < 0 || index >= n {
		index = 0
	}
	return screenshot.GetDisplayBounds(index)
}

// SizeOf retorna largura/altura do monitor informado.
func SizeOf(index int) (int, int) {
	b := boundsAt(index)
	return b.Dx(), b.Dy()
}

// CaptureDisplay captura o monitor informado.
func CaptureDisplay(index int) (image.Image, error) {
	b := boundsAt(index)
	return screenshot.Capture(b.Min.X, b.Min.Y, b.Dx(), b.Dy())
}

// Size retorna o tamanho do monitor principal.
func Size() (int, int) { return SizeOf(0) }

// CaptureImg captura o monitor principal.
func CaptureImg() (image.Image, error) { return CaptureDisplay(0) }

// WriteJPEGDisplay captura um monitor e grava JPEG no writer.
func WriteJPEGDisplay(w io.Writer, index, quality int) error {
	img, err := CaptureDisplay(index)
	if err != nil {
		return err
	}
	return jpeg.Encode(w, img, &jpeg.Options{Quality: quality})
}

// WriteJPEG captura o monitor principal e grava JPEG.
func WriteJPEG(w io.Writer, quality int) error {
	return WriteJPEGDisplay(w, 0, quality)
}
