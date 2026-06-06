# micro (jfunv fork)

> Just use [Neovim](https://justfuckinguseneovim.com).

This is a fork of [micro-editor/micro](https://github.com/micro-editor/micro). For the original README, go read it over there.

---

## Fork changes and features

### Keybindings

| Binding | Action |
|---------|--------|
| `Ctrl+o` | Open file/folder picker |
| `Ctrl+d` | Spawn multi-cursor select |
| `Ctrl+n` | New tab |
| `Ctrl+e` | Command palette |
| `Ctrl+j` / `` Ctrl+` `` | Toggle terminal panel |
| `Ctrl+b` | Toggle file explorer panel |
| `F5` | Run pre-configured workspace command |
| `F6` | Run pre-configured workspace command 6 |
| `F7` | Find |
| `Ctrl+LeftClick` | Go to definition |
| `Ctrl+RightClick` | Add/remove multi-cursor at position |
| `Alt-n` | Spawn multi-cursor |
| `Alt-m` | Spawn multi-cursor select |

### Panels

- **File explorer panel** (`Ctrl+b`) — sidebar file browser
- **Terminal panel** (`Ctrl+j`) — integrated shell in a bottom panel
- **Command palette** (`Ctrl+e`) — fuzzy-searchable command launcher
- All panels support mouse interaction; clicking on an editor pane or tab bar resets panel focus back to the editor

### Workspace config (`.microrc.ini`)

Place a `.microrc.ini` file in a project root to configure workspace-specific behavior:
- `F5` command — binds to `Ctrl+F5` or `F5` key, runs in the terminal panel
- `F6` command — binds to `F6` key

### Go to definition (`Ctrl+LeftClick`)

Ctrl+click on any identifier to jump to its definition. Searches the current buffer first, then other open buffers. Supports patterns for:
- **Go, Rust:** `func`, `fn`
- **Python, JS, Lua:** `def`, `async def`, `function`, `sub`
- **C/C++:** return-type-prefixed functions, `word(...) {` function bodies, `#define`, `typedef`, `struct`, `enum`, `union`
- **Types/classes:** `type`, `class`, `struct`, `enum`, `interface`, `trait`, `module`, `namespace`
- **Variables/constants:** `var`, `const`, `val`, `let`

### Panel focus behavior

Clicking on an editor pane or the tab bar automatically resets panel focus to the editor, so keyboard input goes where you expect.
