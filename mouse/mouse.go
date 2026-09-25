package mouse

import (
	"fmt"
	"sync"

	"github.com/go-vgo/robotgo"
	hook "github.com/robotn/gohook"
)

// State is a snapshot of the mouse's position and button state at a point
// in time.
type State struct {
	X, Y      int
	LeftDown  bool
	RightDown bool
}

type Mouse struct {
	mu      sync.Mutex
	current State
	last    State // state as of the most recent Poll call
	changed bool  // whether current has diverged from last since that call
	events  chan hook.Event
}

func New() *Mouse {
	initial := State{X: -1, Y: -1}
	return &Mouse{current: initial, last: initial}
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

// Poll returns the mouse's current state, whether it has changed since the
// last call to Poll, and (when changed) the state as of that last call.
// When changed is false, last equals current.
func (m *Mouse) Poll() (current State, last State, changed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	current = m.current
	changed = m.changed
	if changed {
		last = m.last
		m.last = m.current
		m.changed = false
	} else {
		last = m.current
	}
	return current, last, changed
}

func (m *Mouse) Move(x, y int) {
	robotgo.Move(x, y)
}

func (m *Mouse) handleEvent(ev hook.Event) {
	switch ev.Kind {
	case hook.MouseMove, hook.MouseDrag:
		x, y := int(ev.X), int(ev.Y)
		m.mu.Lock()
		if x != m.current.X || y != m.current.Y {
			m.current.X, m.current.Y = x, y
			m.changed = true
		}
		m.mu.Unlock()
	case hook.MouseDown:
		m.setButton(ev.Button, true)
	case hook.MouseUp:
		m.setButton(ev.Button, false)
	}
}

func (m *Mouse) setButton(button uint16, down bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch button {
	case hook.MouseMap["left"]:
		if m.current.LeftDown != down {
			m.current.LeftDown = down
			m.changed = true
		}
	case hook.MouseMap["right"]:
		if m.current.RightDown != down {
			m.current.RightDown = down
			m.changed = true
		}
	}
}

func Probe() {
	fmt.Println("Starting ...")

	mouse := New()
	mouse.Start()
	defer mouse.End()
	go mouse.Run()

	for {
		current, _, changed := mouse.Poll()

		if changed {
			fmt.Println(current)
		}
	}
}
