package action

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/micro-editor/micro/v2/internal/buffer"
	"github.com/micro-editor/micro/v2/internal/screen"
	"github.com/micro-editor/tcell/v2"
)

type pickerEntry struct {
	Path string
	Name string
	Dir  bool
}

func (h *BufPane) OpenFilePicker() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	h.showPicker(home)
	return true
}

func (h *BufPane) showPicker(root string) {
	entries := readDirEntries(root)
	lines := make([]string, 0, len(entries)+3)
	lines = append(lines, "  "+root)
	lines = append(lines, "")
	for _, e := range entries {
		if e.Dir {
			lines = append(lines, "  [D] "+e.Name+"/")
		} else {
			lines = append(lines, "     "+e.Name)
		}
	}

	content := strings.Join(lines, "\n")
	buf := buffer.NewBufferFromString(content, root, buffer.BTScratch)
	buf.SetName("Pick File/Folder")
	buf.SetOptionNative("readonly", true)

	pickerPane := h.HSplitBuf(buf)

	pickerPane.PickerRoot = root
	pickerPane.PickerEntries = entries
	pickerPane.PickerIdx = 0
	pickerPane.IsPicker = true
	pickerPane.OrigPane = h

	buf.ClearCursors()
	c := buffer.NewCursor(buf, buffer.Loc{X: 2, Y: 2})
	buf.AddCursor(c)
	buf.SetCurCursor(0)

	Tabs.Resize()
	screen.Redraw()
}

func readDirEntries(dir string) []pickerEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})
	result := make([]pickerEntry, 0, len(entries))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		result = append(result, pickerEntry{
			Path: filepath.Join(dir, e.Name()),
			Name: e.Name(),
			Dir:  e.IsDir(),
		})
	}
	return result
}

func (h *BufPane) PickerHandleEvent(event tcell.Event) {
	switch e := event.(type) {
	case *tcell.EventKey:
		switch e.Key() {
		case tcell.KeyUp:
			h.pickerMove(-1)
		case tcell.KeyDown:
			h.pickerMove(1)
		case tcell.KeyEscape, tcell.KeyCtrlQ:
			h.pickerClose()
		case tcell.KeyEnter:
			h.pickerConfirm()
		case tcell.KeyRight:
			h.pickerEnter()
		case tcell.KeyLeft, tcell.KeyBackspace:
			h.pickerGoUp()
		case tcell.KeyRune:
			switch e.Rune() {
			case 'j':
				h.pickerMove(1)
			case 'k':
				h.pickerMove(-1)
			case 'q':
				h.pickerClose()
			case 'l':
				h.pickerEnter()
			case 'h':
				h.pickerGoUp()
			case '~':
				home, _ := os.UserHomeDir()
				if home != "" {
					h.pickerRefresh(home)
				}
			}
		}
	}
}

func (h *BufPane) pickerMove(delta int) {
	if len(h.PickerEntries) == 0 {
		return
	}
	idx := h.PickerIdx + delta
	if idx < 0 {
		idx = 0
	}
	if idx >= len(h.PickerEntries) {
		idx = len(h.PickerEntries) - 1
	}
	h.PickerIdx = idx
	c := h.Buf.GetActiveCursor()
	c.Y = 2 + h.PickerIdx
	c.X = 2
	h.Cursor.Y = c.Y
	h.Cursor.X = c.X
	h.Relocate()
}

func (h *BufPane) pickerConfirm() {
	if h.PickerIdx < 0 || h.PickerIdx >= len(h.PickerEntries) {
		return
	}
	entry := h.PickerEntries[h.PickerIdx]
	if entry.Dir {
		abs, _ := filepath.Abs(entry.Path)
		os.Chdir(abs)
		if Panels != nil && Panels.Explorer != nil {
			Panels.Explorer.Root = abs
			Panels.Explorer.Refresh()
		}
		h.pickerClose()
		InfoBar.Message("Workspace: " + abs)
		return
	}
	dir := filepath.Dir(entry.Path)
	os.Chdir(dir)
	if Panels != nil && Panels.Explorer != nil {
		Panels.Explorer.Root = dir
		Panels.Explorer.Refresh()
	}
	orig := h.OrigPane
	h.pickerClose()
	if orig != nil {
		b, err := buffer.NewBufferFromFile(entry.Path, buffer.BTDefault)
		if err != nil {
			InfoBar.Error(err)
			return
		}
		orig.OpenBuffer(b)
	}
}

func (h *BufPane) pickerEnter() {
	if h.PickerIdx < 0 || h.PickerIdx >= len(h.PickerEntries) {
		return
	}
	entry := h.PickerEntries[h.PickerIdx]
	if entry.Dir {
		h.pickerRefresh(entry.Path)
		InfoBar.Message(entry.Path)
		return
	}
	h.pickerConfirm()
}

func (h *BufPane) pickerGoUp() {
	parent := filepath.Dir(h.PickerRoot)
	if parent == h.PickerRoot {
		return
	}
	h.pickerRefresh(parent)
}

func (h *BufPane) pickerRefresh(dir string) {
	h.PickerRoot = dir
	h.PickerEntries = readDirEntries(dir)
	h.PickerIdx = 0

	lines := make([]string, 0, len(h.PickerEntries)+3)
	lines = append(lines, "  "+dir)
	lines = append(lines, "")
	for _, e := range h.PickerEntries {
		if e.Dir {
			lines = append(lines, "  [D] "+e.Name+"/")
		} else {
			lines = append(lines, "     "+e.Name)
		}
	}

	content := strings.Join(lines, "\n")
	h.Buf.Replace(buffer.Loc{}, h.Buf.End(), content)
	h.Buf.ClearCursors()
	c := buffer.NewCursor(h.Buf, buffer.Loc{X: 2, Y: 2})
	h.Buf.AddCursor(c)
	h.Buf.SetCurCursor(0)
	h.Cursor = c
	os.Chdir(dir)
	h.Relocate()
	Tabs.Resize()
	screen.Redraw()
}

func (h *BufPane) pickerClose() {
	h.IsPicker = false
	h.Unsplit()
	if Panels != nil && Panels.Explorer != nil {
		Panels.Explorer.Refresh()
	}
	Tabs.Resize()
	screen.Redraw()
}
