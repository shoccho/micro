package action

import (
	"sort"
	"strings"

	"github.com/micro-editor/micro/v2/internal/config"
	"github.com/micro-editor/micro/v2/internal/info"
)

type paletteMatch struct {
	item  info.PaletteItem
	score int
}

func fuzzyScore(query, text string) int {
	query = strings.ToLower(strings.TrimSpace(query))
	text = strings.ToLower(text)
	if query == "" {
		return 0
	}

	score := 0
	last := -1
	for _, qr := range query {
		found := false
		for i, tr := range text {
			if i <= last {
				continue
			}
			if qr == tr {
				found = true
				score += 100 - i
				if i == last+1 {
					score += 25
				}
				last = i
				break
			}
		}
		if !found {
			return -1
		}
	}
	if strings.HasPrefix(text, query) {
		score += 500
	}
	return score
}

func commandBinding(name string) string {
	for key, action := range config.Bindings["buffer"] {
		if action == "command:"+name || action == "command-edit:"+name+" " {
			return key
		}
		if name == "command.palette" && action == "CommandPalette" {
			return key
		}
		if name == "explorer.toggle" && action == "ToggleExplorer" {
			return key
		}
		if name == "terminal.toggle" && action == "ToggleTerminal" {
			return key
		}
	}
	return ""
}

func commandPaletteItems(query string) []info.PaletteItem {
	matches := make([]paletteMatch, 0, len(commands))
	for name, cmd := range commands {
		search := strings.Join([]string{name, cmd.title, cmd.description, cmd.category}, " ")
		score := fuzzyScore(query, search)
		if score < 0 {
			continue
		}
		matches = append(matches, paletteMatch{
			item: info.PaletteItem{
				ID:          name,
				Title:       cmd.title,
				Description: cmd.description,
				Binding:     commandBinding(name),
			},
			score: score,
		})
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score == matches[j].score {
			return matches[i].item.Title < matches[j].item.Title
		}
		return matches[i].score > matches[j].score
	})

	items := make([]info.PaletteItem, len(matches))
	for i, m := range matches {
		items[i] = m.item
	}
	return items
}

func commandNeedsInput(name string) bool {
	switch name {
	case "set", "setlocal", "toggle", "togglelocal", "reset", "show", "run", "bind", "unbind", "goto", "jump", "replace", "replaceall", "vsplit", "hsplit", "tab", "help", "plugin", "cd", "open", "tabmove", "tabswitch", "term", "textfilter":
		return true
	}
	return false
}

func (h *BufPane) openCommandPrompt(prefill string) {
	InfoBar.Prompt("> ", prefill, "Command", nil, func(resp string, canceled bool) {
		if !canceled {
			h.HandleCommand(resp)
		}
	})
}

// CommandPalette opens a VS Code-style command palette.
func (h *BufPane) CommandPalette() bool {
	update := func(resp string) {
		InfoBar.PaletteItems = commandPaletteItems(resp)
		if InfoBar.PaletteIndex >= len(InfoBar.PaletteItems) {
			InfoBar.PaletteIndex = len(InfoBar.PaletteItems) - 1
		}
		if InfoBar.PaletteIndex < 0 {
			InfoBar.PaletteIndex = 0
		}
	}

	InfoBar.Prompt("Command: ", "", "CommandPalette", update, func(resp string, canceled bool) {
		items := InfoBar.PaletteItems
		idx := InfoBar.PaletteIndex
		InfoBar.PaletteActive = false
		InfoBar.PaletteItems = nil
		InfoBar.PaletteIndex = 0

		if canceled {
			return
		}

		trimmed := strings.TrimSpace(resp)
		if trimmed != "" {
			parts := strings.Fields(trimmed)
			if len(parts) > 0 {
				if _, ok := commands[parts[0]]; ok && strings.Contains(trimmed, " ") {
					h.HandleCommand(trimmed)
					return
				}
			}
		}

		if len(items) == 0 || idx < 0 || idx >= len(items) {
			return
		}

		name := items[idx].ID
		if commandNeedsInput(name) {
			h.openCommandPrompt(name + " ")
			return
		}
		h.HandleCommand(name)
	})
	update("")
	return true
}

func (h *BufPane) CommandPaletteCmd(args []string) { h.CommandPalette() }
func (h *BufPane) CommandPromptCmd(args []string)  { h.CommandMode() }
