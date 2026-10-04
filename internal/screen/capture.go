package screen

import (
	"image"
	"image/jpeg"
	"io"

	"github.com/kbinani/screenshot"
)

// Bounds retorna a região do display principal.
func Bounds() (x, y, w, h int) {
	n := screenshot.NumActiveDisplays()
	if n <= 0 {
		return 0, 0, 1, 1
	}
	b := screenshot.GetDisplayBounds(0)
	return b.Min.X, b.Min.Y, b.Dx(), b.Dy()
}

// Size retorna a largura e altura do display principal.
func Size() (int, int) {
	_, _, w, h := Bounds()
	return w, h
}

// CaptureImg captura o display principal como imagem.
func CaptureImg() (image.Image, error) {
	x, y, w, h := Bounds()
	return screenshot.Capture(x, y, w, h)
}

// WriteJPEG captura e codifica a tela em JPEG no writer.
func WriteJPEG(w io.Writer, quality int) error {
	img, err := CaptureImg()
	if err != nil {
		return err
	}
	return jpeg.Encode(w, img, &jpeg.Options{Quality: quality})
}
