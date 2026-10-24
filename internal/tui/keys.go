package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Tab      key.Binding
	Filter   key.Binding
	Search   key.Binding
	Edit     key.Binding
	Comment  key.Binding
	Diff     key.Binding
	DiffDisk key.Binding
	Lint     key.Binding
	Copy     key.Binding
	Delete   key.Binding
	Rename   key.Binding
	Path     key.Binding
	Update   key.Binding
	Select   key.Binding
	New      key.Binding
	Revert   key.Binding
	Hide     key.Binding
	Pending  key.Binding
	Settings key.Binding
	Help     key.Binding
	Quit     key.Binding
}

func newKeys() keyMap {
	return keyMap{
		Up:       key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
		Down:     key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
		Tab:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "focus")),
		Filter:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter list")),
		Search:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search file")),
		Edit:     key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		Comment:  key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "comment")),
		Diff:     key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "diff previous")),
		DiffDisk: key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "diff disk")),
		Lint:     key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "lint")),
		Copy:     key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy")),
		Delete:   key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "delete")),
		Rename:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rename")),
		Path:     key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "edit path")),
		Update:   key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "update")),
		Select:   key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "select")),
		New:      key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "new env")),
		Revert:   key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "revert")),
		Hide:     key.NewBinding(key.WithKeys("H"), key.WithHelp("H", "hide")),
		Pending:  key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "pending")),
		Settings: key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "roots")),
		Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Tab, k.Filter, k.Select, k.Edit, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Tab, k.Filter, k.Search},
		{k.Edit, k.Comment, k.Delete, k.Rename, k.Path, k.Hide},
		{k.Diff, k.DiffDisk, k.Lint, k.Copy},
		{k.Update, k.Select, k.New, k.Revert, k.Pending, k.Settings, k.Help, k.Quit},
	}
}
