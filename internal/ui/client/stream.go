package clientui

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"image/jpeg"
	"io"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"

	"github.com/victorbillyph/fvremote/internal/net/client"
)

// fetchStream lê frames JPEG com prefixo de tamanho e atualiza o canvas.
func fetchStream(c *client.Client, screen *remoteCanvas, w fyne.Window) {
	resp, err := c.HTTP.Get(c.Base + "/stream")
	if err != nil {
		fyne.Do(func() { dialog.ShowError(err, w) })
		return
	}
	defer resp.Body.Close()

	r := bufio.NewReader(resp.Body)
	var hdr [4]byte
	for {
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			return
		}
		n := binary.BigEndian.Uint32(hdr[:])
		if n == 0 || n > 64<<20 {
			return
		}
		data := make([]byte, n)
		if _, err := io.ReadFull(r, data); err != nil {
			return
		}
		img, err := jpeg.Decode(bytes.NewReader(data))
		if err != nil {
			continue
		}
		fyne.Do(func() {
			screen.img.Image = img
			screen.img.Refresh()
		})
	}
}
