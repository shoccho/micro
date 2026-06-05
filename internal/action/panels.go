package action

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	runewidth "github.com/mattn/go-runewidth"
	"github.com/micro-editor/micro/v2/internal/buffer"
	"github.com/micro-editor/micro/v2/internal/clipboard"
	"github.com/micro-editor/micro/v2/internal/config"
	"github.com/micro-editor/micro/v2/internal/display"
	"github.com/micro-editor/micro/v2/internal/screen"
	"github.com/micro-editor/micro/v2/internal/shell"
	"github.com/micro-editor/micro/v2/internal/util"
	"github.com/micro-editor/tcell/v2"
	"github.com/micro-editor/terminal"
)

const (
	panelFocusEditor   = ""
	panelFocusExplorer = "explorer"
	panelFocusTerminal = "terminal"
)

type panelRect struct {
	X, Y, W, H int
}

// Panels owns global IDE-style panels that sit outside the tab split tree.
var Panels *PanelManager

type PanelManager struct {
	Explorer *ExplorerPanel
	Terminal *TerminalPanel

	focus string

	resizingExplorer bool
	resizingTerminal bool
}

func NewPanelManager() *PanelManager {
	return &PanelManager{
		Explorer: NewExplorerPanel(),
		Terminal: NewTerminalPanel(),
	}
}

func clampPanel(v, min, max int) int {
	if max < min {
		return min
	}
	return util.Clamp(v, min, max)
}

func (p *PanelManager) rects() (panelRect, panelRect, panelRect) {
	w, h := screen.Screen.Size()
	infoOffset := config.GetInfoBarOffset()
	availableH := util.Max(1, h-infoOffset)

	explorer := panelRect{X: 0, Y: 0, W: 0, H: availableH}
	editor := panelRect{X: 0, Y: 0, W: w, H: availableH}
	term := panelRect{X: 0, Y: availableH, W: 0, H: 0}

	if p != nil && p.Explorer != nil && p.Explorer.Visible {
		maxExplorer := util.Max(1, w-2)
		minExplorer := util.Min(18, maxExplorer)
		p.Explorer.Width = clampPanel(p.Explorer.Width, minExplorer, maxExplorer)
		explorer.W = p.Explorer.Width
		editor.X = explorer.W + 1
		editor.W = util.Max(1, w-editor.X)
	}

	if p != nil && p.Terminal != nil && p.Terminal.Visible && availableH > 3 {
		maxTermH := util.Max(1, availableH-2)
		minTermH := util.Min(3, maxTermH)
		p.Terminal.Height = clampPanel(p.Terminal.Height, minTermH, maxTermH)
		editor.H = util.Max(1, availableH-p.Terminal.Height-1)
		term = panelRect{X: editor.X, Y: editor.Y + editor.H + 1, W: editor.W, H: p.Terminal.Height}
	}

	return explorer, editor, term
}

func EditorRect() panelRect {
	if Panels == nil {
		w, h := screen.Screen.Size()
		return panelRect{X: 0, Y: 0, W: w, H: util.Max(1, h-config.GetInfoBarOffset())}
	}
	_, editor, _ := Panels.rects()
	return editor
}

func (p *PanelManager) resizeTabs() {
	if Tabs != nil {
		Tabs.Resize()
	}
}

func (p *PanelManager) Display() {
	if p == nil {
		return
	}
	explorer, _, term := p.rects()
	if p.Explorer != nil && p.Explorer.Visible {
		p.Explorer.Display(explorer, p.focus == panelFocusExplorer)
	}
	if p.Terminal != nil && p.Terminal.Visible {
		p.Terminal.Display(term, p.focus == panelFocusTerminal)
	}
}

func (p *PanelManager) HandleEvent(event tcell.Event) bool {
	if p == nil {
		return false
	}

	explorer, _, term := p.rects()

	switch e := event.(type) {
	case *tcell.EventResize:
		p.resizeTabs()
		return false
	case *tcell.EventMouse:
		mx, my := e.Position()
		btn := e.Buttons()
		if btn == tcell.ButtonNone {
			wasResizing := p.resizingExplorer || p.resizingTerminal
			p.resizingExplorer = false
			p.resizingTerminal = false
			if !wasResizing && p.focus == panelFocusTerminal && p.Terminal != nil && p.Terminal.Visible {
				p.Terminal.HandleEvent(e, term)
				return true
			}
			return wasResizing
		}

		if p.resizingExplorer && btn == tcell.Button1 {
			w, _ := screen.Screen.Size()
			maxExplorer := util.Max(1, w-2)
			minExplorer := util.Min(18, maxExplorer)
			p.Explorer.Width = clampPanel(mx, minExplorer, maxExplorer)
			p.resizeTabs()
			return true
		}
		if p.resizingTerminal && btn == tcell.Button1 {
			_, h := screen.Screen.Size()
			availableH := util.Max(1, h-config.GetInfoBarOffset())
			maxTermH := util.Max(1, availableH-2)
			minTermH := util.Min(3, maxTermH)
			p.Terminal.Height = clampPanel(availableH-my-1, minTermH, maxTermH)
			p.resizeTabs()
			return true
		}

		if p.Explorer != nil && p.Explorer.Visible && mx == explorer.X+explorer.W && my >= explorer.Y && my < explorer.Y+explorer.H && btn == tcell.Button1 {
			p.resizingExplorer = true
			return true
		}
		if p.Terminal != nil && p.Terminal.Visible && my == term.Y-1 && mx >= term.X && mx < term.X+term.W && btn == tcell.Button1 {
			p.resizingTerminal = true
			return true
		}

		if p.Explorer != nil && p.Explorer.Visible && mx >= explorer.X && mx < explorer.X+explorer.W && my >= explorer.Y && my < explorer.Y+explorer.H {
			p.focus = panelFocusExplorer
			p.Explorer.HandleEvent(e, explorer)
			return true
		}
		if p.Terminal != nil && p.Terminal.Visible && mx >= term.X && mx < term.X+term.W && my >= term.Y && my < term.Y+term.H {
			p.focus = panelFocusTerminal
			p.Terminal.HandleEvent(e, term)
			return true
		}
	case *tcell.EventKey, *tcell.EventPaste:
		if p.focus == panelFocusExplorer && p.Explorer != nil && p.Explorer.Visible {
			if p.Explorer.HandleEvent(event, explorer) {
				return true
			}
		}
		if p.focus == panelFocusTerminal && p.Terminal != nil && p.Terminal.Visible {
			p.Terminal.HandleEvent(event, term)
			return true
		}
	}

	return false
}

func (p *PanelManager) CloseTerms() {
	if p != nil && p.Terminal != nil && p.Terminal.Terminal != nil && p.Terminal.Terminal.Status == shell.TTClose {
		p.Terminal.Terminal = nil
		p.Terminal.Window = nil
		p.Terminal.Visible = false
		p.focus = panelFocusEditor
		p.resizeTabs()
	}
}

func (p *PanelManager) ToggleExplorer() {
	if p == nil || p.Explorer == nil {
		return
	}
	p.Explorer.Visible = !p.Explorer.Visible
	if p.Explorer.Visible {
		p.focus = panelFocusExplorer
		p.Explorer.Refresh()
	} else if p.focus == panelFocusExplorer {
		p.focus = panelFocusEditor
	}
	p.resizeTabs()
}

func (p *PanelManager) ToggleTerminal() {
	if p == nil || p.Terminal == nil {
		return
	}
	if p.Terminal.Visible {
		p.Terminal.Visible = false
		if p.focus == panelFocusTerminal {
			p.focus = panelFocusEditor
		}
		p.resizeTabs()
		return
	}
	if err := p.Terminal.EnsureStarted(); err != nil {
		InfoBar.Error(err)
		return
	}
	p.Terminal.Visible = true
	p.focus = panelFocusTerminal
	p.resizeTabs()
}

func (p *PanelManager) FocusExplorer() {
	if p == nil || p.Explorer == nil {
		return
	}
	if !p.Explorer.Visible {
		p.Explorer.Visible = true
		p.Explorer.Refresh()
		p.resizeTabs()
	}
	p.focus = panelFocusExplorer
}

func (p *PanelManager) FocusTerminal() {
	if p == nil || p.Terminal == nil {
		return
	}
	if !p.Terminal.Visible {
		p.ToggleTerminal()
		return
	}
	p.focus = panelFocusTerminal
}

func drawPanelText(x, y, width int, text string, style tcell.Style) {
	xpos := x
	for _, r := range text {
		rw := runewidth.RuneWidth(r)
		if xpos+rw > x+width {
			break
		}
		for j := 0; j < rw; j++ {
			c := r
			if j > 0 {
				c = ' '
			}
			screen.SetContent(xpos, y, c, nil, style)
			xpos++
		}
	}
	for xpos < x+width {
		screen.SetContent(xpos, y, ' ', nil, style)
		xpos++
	}
}

type explorerEntry struct {
	Path  string
	Name  string
	Depth int
	Dir   bool
}

type ExplorerPanel struct {
	Visible bool
	Width   int
	Root    string

	entries  []explorerEntry
	expanded map[string]bool
	selected int
	scroll   int

	lastClickPath string
	lastClickTime time.Time
}

func NewExplorerPanel() *ExplorerPanel {
	wd, err := os.Getwd()
	if err != nil || wd == "" {
		wd = "."
	}
	width := 30
	if v, ok := config.GlobalSettings["explorerwidth"].(float64); ok {
		width = int(v)
	}
	e := &ExplorerPanel{
		Visible:  false,
		Width:    width,
		Root:     wd,
		expanded: make(map[string]bool),
	}
	e.Refresh()
	return e
}

func (e *ExplorerPanel) Refresh() {
	if e == nil {
		return
	}
	e.entries = e.entries[:0]
	e.walk(e.Root, 0)
	if e.selected >= len(e.entries) {
		e.selected = len(e.entries) - 1
	}
	if e.selected < 0 {
		e.selected = 0
	}
}

func (e *ExplorerPanel) walk(dir string, depth int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})
	for _, de := range entries {
		name := de.Name()
		path := filepath.Join(dir, name)
		entry := explorerEntry{Path: path, Name: name, Depth: depth, Dir: de.IsDir()}
		e.entries = append(e.entries, entry)
		if entry.Dir && e.expanded[path] {
			e.walk(path, depth+1)
		}
	}
}

func (e *ExplorerPanel) rowHeight(rect panelRect) int {
	return util.Max(0, rect.H-2)
}

func (e *ExplorerPanel) ensureVisible(rect panelRect) {
	rows := e.rowHeight(rect)
	if rows <= 0 {
		return
	}
	if e.selected < e.scroll {
		e.scroll = e.selected
	} else if e.selected >= e.scroll+rows {
		e.scroll = e.selected - rows + 1
	}
	if e.scroll < 0 {
		e.scroll = 0
	}
}

func (e *ExplorerPanel) Display(rect panelRect, focused bool) {
	if rect.W <= 0 || rect.H <= 0 {
		return
	}
	e.ensureVisible(rect)

	style := config.DefStyle
	activeStyle := style.Reverse(true)
	headerStyle := config.DefStyle.Reverse(true)
	if style, ok := config.Colorscheme["statusline"]; ok {
		headerStyle = style
	}

	drawPanelText(rect.X, rect.Y, rect.W, "EXPLORER", headerStyle)
	drawPanelText(rect.X, rect.Y+1, rect.W, filepath.Base(e.Root), headerStyle)

	rows := e.rowHeight(rect)
	for row := 0; row < rows; row++ {
		idx := e.scroll + row
		y := rect.Y + 2 + row
		lineStyle := config.DefStyle
		text := ""
		if idx < len(e.entries) {
			entry := e.entries[idx]
			prefix := "  "
			if entry.Dir {
				if e.expanded[entry.Path] {
					prefix = "v "
				} else {
					prefix = "> "
				}
			}
			text = strings.Repeat("  ", entry.Depth) + prefix + entry.Name
			if entry.Dir {
				text += "/"
			}
			if idx == e.selected {
				lineStyle = activeStyle
			}
		}
		drawPanelText(rect.X, y, rect.W, text, lineStyle)
	}

	dividerStyle := config.DefStyle.Reverse(true)
	for y := rect.Y; y < rect.Y+rect.H; y++ {
		screen.SetContent(rect.X+rect.W, y, '|', nil, dividerStyle)
	}
}

func (e *ExplorerPanel) HandleEvent(event tcell.Event, rect panelRect) bool {
	switch ev := event.(type) {
	case *tcell.EventKey:
		return e.handleKey(ev)
	case *tcell.EventMouse:
		e.handleMouse(ev, rect)
		return true
	default:
		return false
	}
}

func (e *ExplorerPanel) handleKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyUp:
		e.move(-1)
		return true
	case tcell.KeyDown:
		e.move(1)
		return true
	case tcell.KeyPgUp:
		e.move(-10)
		return true
	case tcell.KeyPgDn:
		e.move(10)
		return true
	case tcell.KeyEnter, tcell.KeyTab:
		e.openSelected()
		return true
	case tcell.KeyRight:
		e.expandSelected()
		return true
	case tcell.KeyLeft:
		e.collapseSelected()
		return true
	case tcell.KeyDelete:
		e.deleteSelected()
		return true
	case tcell.KeyEsc:
		if Panels != nil {
			Panels.focus = panelFocusEditor
			Panels.resizeTabs()
		}
		return true
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'q':
			e.Visible = false
			if Panels != nil {
				Panels.focus = panelFocusEditor
				Panels.resizeTabs()
			}
			return true
		case 'r':
			e.renameSelected()
			return true
		case 'n':
			e.createSelected(false)
			return true
		case 'N':
			e.createSelected(true)
			return true
		case 'd':
			e.deleteSelected()
			return true
		case 'R':
			e.Refresh()
			return true
		case 'l':
			e.openSelected()
			return true
		case 'h':
			e.collapseSelected()
			return true
		case 'j':
			e.move(1)
			return true
		case 'k':
			e.move(-1)
			return true
		}
	}
	return false
}

func (e *ExplorerPanel) handleMouse(ev *tcell.EventMouse, rect panelRect) {
	mx, my := ev.Position()
	if ev.Buttons() == tcell.WheelUp {
		e.move(-1)
		return
	}
	if ev.Buttons() == tcell.WheelDown {
		e.move(1)
		return
	}
	if ev.Buttons() != tcell.Button1 || my < rect.Y+2 {
		return
	}
	idx := e.scroll + (my - rect.Y - 2)
	if idx < 0 || idx >= len(e.entries) || mx < rect.X || mx >= rect.X+rect.W {
		return
	}
	e.selected = idx
	entry := e.entries[idx]
	if e.lastClickPath == entry.Path && time.Since(e.lastClickTime)/time.Millisecond < config.DoubleClickThreshold {
		e.openSelected()
		e.lastClickPath = ""
		return
	}
	e.lastClickPath = entry.Path
	e.lastClickTime = time.Now()
}

func (e *ExplorerPanel) move(delta int) {
	if len(e.entries) == 0 {
		return
	}
	e.selected = util.Clamp(e.selected+delta, 0, len(e.entries)-1)
}

func (e *ExplorerPanel) selectedEntry() (explorerEntry, bool) {
	if e.selected < 0 || e.selected >= len(e.entries) {
		return explorerEntry{}, false
	}
	return e.entries[e.selected], true
}

func currentEditorPane() *BufPane {
	if Tabs == nil || len(Tabs.List) == 0 {
		return nil
	}
	if p := MainTab().CurPane(); p != nil {
		return p
	}
	for _, p := range MainTab().Panes {
		if bp, ok := p.(*BufPane); ok {
			return bp
		}
	}
	return nil
}

func (e *ExplorerPanel) openSelected() {
	entry, ok := e.selectedEntry()
	if !ok {
		return
	}
	if entry.Dir {
		if e.expanded[entry.Path] {
			delete(e.expanded, entry.Path)
		} else {
			e.expanded[entry.Path] = true
		}
		e.Refresh()
		return
	}
	bp := currentEditorPane()
	if bp == nil {
		InfoBar.Error("No editor pane available")
		return
	}
	b, err := buffer.NewBufferFromFile(entry.Path, buffer.BTDefault)
	if err != nil {
		InfoBar.Error(err)
		return
	}
	bp.OpenBuffer(b)
}

func (e *ExplorerPanel) expandSelected() {
	entry, ok := e.selectedEntry()
	if ok && entry.Dir {
		e.expanded[entry.Path] = true
		e.Refresh()
	}
}

func (e *ExplorerPanel) collapseSelected() {
	entry, ok := e.selectedEntry()
	if !ok || !entry.Dir {
		return
	}
	if e.expanded[entry.Path] {
		delete(e.expanded, entry.Path)
		e.Refresh()
	}
}

func (e *ExplorerPanel) targetDir() string {
	entry, ok := e.selectedEntry()
	if !ok {
		return e.Root
	}
	if entry.Dir {
		return entry.Path
	}
	return filepath.Dir(entry.Path)
}

func (e *ExplorerPanel) createSelected(dir bool) {
	prompt := "New file: "
	if dir {
		prompt = "New folder: "
	}
	InfoBar.Prompt(prompt, "", "ExplorerCreate", nil, func(name string, canceled bool) {
		if canceled || strings.TrimSpace(name) == "" {
			return
		}
		path := filepath.Join(e.targetDir(), name)
		if _, err := os.Stat(path); err == nil {
			InfoBar.Error("Path already exists: ", path)
			return
		}
		var err error
		if dir {
			err = os.Mkdir(path, os.ModePerm)
		} else {
			var f *os.File
			f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
			if f != nil {
				f.Close()
			}
		}
		if err != nil {
			InfoBar.Error(err)
			return
		}
		e.Refresh()
		InfoBar.Message("Created ", path)
	})
}

func (e *ExplorerPanel) renameSelected() {
	entry, ok := e.selectedEntry()
	if !ok {
		return
	}
	InfoBar.Prompt("Rename to: ", entry.Name, "ExplorerRename", nil, func(name string, canceled bool) {
		if canceled || strings.TrimSpace(name) == "" || name == entry.Name {
			return
		}
		newPath := filepath.Join(filepath.Dir(entry.Path), name)
		if err := os.Rename(entry.Path, newPath); err != nil {
			InfoBar.Error(err)
			return
		}
		if e.expanded[entry.Path] {
			delete(e.expanded, entry.Path)
			e.expanded[newPath] = true
		}
		e.Refresh()
		InfoBar.Message("Renamed to ", name)
	})
}

func (e *ExplorerPanel) deleteSelected() {
	entry, ok := e.selectedEntry()
	if !ok {
		return
	}
	InfoBar.YNPrompt("Delete "+entry.Name+"? (y,n,esc)", func(yes, canceled bool) {
		if canceled || !yes {
			return
		}
		if err := os.RemoveAll(entry.Path); err != nil {
			InfoBar.Error(err)
			return
		}
		delete(e.expanded, entry.Path)
		e.Refresh()
		InfoBar.Message("Deleted ", entry.Name)
	})
}

type TerminalPanel struct {
	Visible bool
	Height  int

	*shell.Terminal
	Window *display.TermWindow

	mouseReleased bool
}

func NewTerminalPanel() *TerminalPanel {
	height := 12
	if v, ok := config.GlobalSettings["terminalheight"].(float64); ok {
		height = int(v)
	}
	return &TerminalPanel{Height: height, mouseReleased: true}
}

func (t *TerminalPanel) EnsureStarted() error {
	if !TermEmuSupported {
		return errors.New("terminal emulator is not supported on this system")
	}
	if t.Terminal != nil && t.Terminal.Status != shell.TTClose {
		return nil
	}
	sh := os.Getenv("SHELL")
	if sh == "" {
		sh = "sh"
	}
	term := new(shell.Terminal)
	if err := term.Start([]string{sh}, false, true, nil, nil); err != nil {
		return err
	}
	t.Terminal = term
	t.Window = display.NewTermWindow(0, 0, 1, 1, term)
	return nil
}

func (t *TerminalPanel) Resize(rect panelRect) {
	if t.Terminal == nil || t.Window == nil || rect.W <= 0 || rect.H <= 0 {
		return
	}
	v := t.Window.GetView()
	v.X, v.Y = rect.X, rect.Y
	t.Window.SetView(v)
	t.Window.Resize(rect.W, rect.H)
}

func (t *TerminalPanel) Display(rect panelRect, focused bool) {
	if rect.W <= 0 || rect.H <= 0 {
		return
	}
	if err := t.EnsureStarted(); err != nil {
		drawPanelText(rect.X, rect.Y, rect.W, fmt.Sprint(err), config.DefStyle.Reverse(true))
		return
	}
	dividerStyle := config.DefStyle.Reverse(true)
	for x := rect.X; x < rect.X+rect.W; x++ {
		screen.SetContent(x, rect.Y-1, '-', nil, dividerStyle)
	}
	t.Resize(rect)
	t.Window.SetActive(focused)
	t.Window.Display()
}

func (t *TerminalPanel) HandleEvent(event tcell.Event, rect panelRect) {
	if t.Terminal == nil {
		return
	}
	switch e := event.(type) {
	case *tcell.EventKey:
		ke := keyEvent(e)
		if ke.Name() == "Ctrl-j" || ke.Name() == "Ctrl-J" {
			if Panels != nil {
				Panels.ToggleTerminal()
			}
			return
		}
		if t.Status == shell.TTDone {
			switch e.Key() {
			case tcell.KeyEsc, tcell.KeyCtrlQ, tcell.KeyEnter:
				t.Terminal.Close()
				t.Terminal = nil
				t.Window = nil
				t.Visible = false
				if Panels != nil {
					Panels.focus = panelFocusEditor
					Panels.resizeTabs()
				}
			}
			return
		}
		if e.Key() == tcell.KeyCtrlC && t.HasSelection() {
			clipboard.Write(t.GetSelection(rect.W), clipboard.ClipboardReg)
			InfoBar.Message("Copied selection to clipboard")
		} else if t.Status != shell.TTDone {
			t.WriteString(event.EscSeq())
		}
	case *tcell.EventPaste:
		if t.Status != shell.TTDone {
			t.WriteString(event.EscSeq())
		}
	case *tcell.EventMouse:
		if t.State.Mode(terminal.ModeMouseMask) {
			return
		}
		x, y := e.Position()
		x -= rect.X
		y -= rect.Y
		if x < 0 || x >= rect.W || y < 0 || y >= rect.H {
			return
		}
		if e.Buttons() == tcell.Button1 {
			if !t.mouseReleased {
				t.Selection[1].X = x
				t.Selection[1].Y = y
			} else {
				t.Selection[0].X = x
				t.Selection[0].Y = y
				t.Selection[1].X = x
				t.Selection[1].Y = y
			}
			t.mouseReleased = false
		} else if e.Buttons() == tcell.ButtonNone {
			if !t.mouseReleased {
				t.Selection[1].X = x
				t.Selection[1].Y = y
			}
			t.mouseReleased = true
		}
	}
}

func (h *BufPane) ToggleExplorer() bool {
	if Panels != nil {
		Panels.ToggleExplorer()
	}
	return true
}

func (h *BufPane) FocusExplorer() bool {
	if Panels != nil {
		Panels.FocusExplorer()
	}
	return true
}

func (h *BufPane) ToggleTerminal() bool {
	if Panels != nil {
		Panels.ToggleTerminal()
	}
	return true
}

func (h *BufPane) FocusTerminal() bool {
	if Panels != nil {
		Panels.FocusTerminal()
	}
	return true
}

func (h *BufPane) ExplorerNewFile() bool {
	if Panels != nil && Panels.Explorer != nil {
		Panels.FocusExplorer()
		Panels.Explorer.createSelected(false)
	}
	return true
}

func (h *BufPane) ExplorerNewFolder() bool {
	if Panels != nil && Panels.Explorer != nil {
		Panels.FocusExplorer()
		Panels.Explorer.createSelected(true)
	}
	return true
}

func (h *BufPane) ExplorerRename() bool {
	if Panels != nil && Panels.Explorer != nil {
		Panels.FocusExplorer()
		Panels.Explorer.renameSelected()
	}
	return true
}

func (h *BufPane) ExplorerDelete() bool {
	if Panels != nil && Panels.Explorer != nil {
		Panels.FocusExplorer()
		Panels.Explorer.deleteSelected()
	}
	return true
}

func (h *BufPane) ToggleExplorerCmd(args []string)    { h.ToggleExplorer() }
func (h *BufPane) FocusExplorerCmd(args []string)     { h.FocusExplorer() }
func (h *BufPane) ToggleTerminalCmd(args []string)    { h.ToggleTerminal() }
func (h *BufPane) FocusTerminalCmd(args []string)     { h.FocusTerminal() }
func (h *BufPane) ExplorerNewFileCmd(args []string)   { h.ExplorerNewFile() }
func (h *BufPane) ExplorerNewFolderCmd(args []string) { h.ExplorerNewFolder() }
func (h *BufPane) ExplorerRenameCmd(args []string)    { h.ExplorerRename() }
func (h *BufPane) ExplorerDeleteCmd(args []string)    { h.ExplorerDelete() }
