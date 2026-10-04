package input

import "github.com/go-vgo/robotgo"

func Move(x, y int) {
	robotgo.Move(x, y)
}

func Click(left, down bool) {
	btn := "left"
	if !left {
		btn = "right"
	}
	if down {
		_ = robotgo.MouseDown(btn)
		return
	}
	_ = robotgo.MouseUp(btn)
}

func Scroll(delta int) {
	robotgo.Scroll(0, delta)
}

func Key(name string, down bool) {
	if down {
		robotgo.KeyDown(name)
		return
	}
	robotgo.KeyUp(name)
}
