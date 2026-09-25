package mouse

import (
	"fmt"

	hook "github.com/robotn/gohook"
)

type Mouse struct {
	X, Y      int
	LeftDown  bool
	RightDown bool
	events    chan hook.Event
}

func New() *Mouse {
	return &Mouse{X: -1, Y: -1}
}

// Start begins listening for global mouse events.
func (m *Mouse) Start() {
	m.events = hook.Start()
}

// End stops listening for mouse events.
func (m *Mouse) End() {
	hook.End()
}

// Run blocks, processing events as they arrive, until the event channel closes.
func (m *Mouse) Run() {
	for ev := range m.events {
		m.handleEvent(ev)
	}
}

func (m *Mouse) handleEvent(ev hook.Event) {
	switch ev.Kind {
	case hook.MouseMove, hook.MouseDrag:
		x, y := int(ev.X), int(ev.Y)
		if x != m.X || y != m.Y {
			m.X, m.Y = x, y
			fmt.Printf("Mouse position: %d, %d\n", m.X, m.Y)
		}
	case hook.MouseDown:
		m.setButton(ev.Button, true)
	case hook.MouseUp:
		m.setButton(ev.Button, false)
	}
}

func (m *Mouse) setButton(button uint16, down bool) {
	state := "UP"
	if down {
		state = "DOWN"
	}

	switch button {
	case hook.MouseMap["left"]:
		m.LeftDown = down
		fmt.Printf("Left button %s\n", state)
	case hook.MouseMap["right"]:
		m.RightDown = down
		fmt.Printf("Right button %s\n", state)
	}
}
