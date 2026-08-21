// Interactive prompts for the init wizard, built on bubbletea and the
// bubbles component library.
//
// Every prompt here has a plain-text twin in init.go. The wizard picks
// between them by whether stdin is a terminal, because init has to keep
// working where there is none: a test driving the command in process, a
// CI job, a shell pipeline. A prompt that only renders is a prompt that
// cannot be scripted.
package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true)
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	choiceStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	answerStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	noteStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true)
	errStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

// choiceItem is one option with the sentence explaining when to pick it.
type choiceItem struct {
	label string
	help  string
}

func (c choiceItem) FilterValue() string { return c.label }
func (c choiceItem) Title() string       { return c.label }
func (c choiceItem) Description() string { return c.help }

// choiceDelegate renders an option over two lines: the label, then why
// you would choose it. The reason is the useful half, so it is never
// truncated away — the list is short enough that it always fits.
type choiceDelegate struct{ width int }

func (d choiceDelegate) Height() int                         { return 2 }
func (d choiceDelegate) Spacing() int                        { return 1 }
func (d choiceDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }
func (d choiceDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	c, ok := item.(choiceItem)
	if !ok {
		return
	}
	cursor, label := "  ", choiceStyle.Render(c.label)
	if index == m.Index() {
		cursor, label = selectedStyle.Render("› "), selectedStyle.Render(c.label)
	}
	fmt.Fprintf(w, "%s%s\n    %s", cursor, label, helpStyle.Render(c.help))
}

// newChoiceList builds the question. Separate from running it so the
// rendering can be tested without a terminal.
func newChoiceList(prompt string, options []string, def int, help map[string]string) list.Model {
	items := make([]list.Item, 0, len(options))
	for _, o := range options {
		items = append(items, choiceItem{label: o, help: help[o]})
	}
	// A zero width leaves the title with nothing to render into, so the
	// list is sized from what it actually has to show.
	width := utf8.RuneCountInString(prompt)
	for _, o := range options {
		width = max(width, utf8.RuneCountInString(o)+2, utf8.RuneCountInString(help[o])+4)
	}
	l := list.New(items, choiceDelegate{}, width+4, len(options)*3+3)
	l.Title = prompt
	l.Styles.Title = titleStyle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.Select(def)
	return l
}

// choiceModel is one question.
type choiceModel struct {
	list     list.Model
	chosen   string
	quitting bool
}

func (m choiceModel) Init() tea.Cmd { return nil }

func (m choiceModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			if c, ok := m.list.SelectedItem().(choiceItem); ok {
				m.chosen = c.label
			}
			m.quitting = true
			return m, tea.Quit
		case "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m choiceModel) View() string {
	if m.quitting {
		return ""
	}
	return m.list.View()
}

// askChoice runs one selection prompt and returns the chosen label. An
// empty return means the prompt was abandoned.
func (w *wizard) askChoice(prompt string, options []string, def int, help map[string]string) string {
	final, err := tea.NewProgram(
		choiceModel{list: newChoiceList(prompt, options, def, help)},
		tea.WithOutput(w.out),
	).Run()
	if err != nil {
		return ""
	}
	chosen := final.(choiceModel).chosen
	if chosen != "" {
		w.answered(prompt, chosen)
	}
	return chosen
}

// textModel is a single-line answer with a default.
type textModel struct {
	input    textinput.Model
	prompt   string
	def      string
	validate func(string) error
	err      error
	answer   string
	done     bool
}

func (m textModel) Init() tea.Cmd { return textinput.Blink }

func (m textModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			answer := strings.TrimSpace(m.input.Value())
			if answer == "" {
				answer = m.def
			}
			if m.validate != nil {
				if err := m.validate(answer); err != nil {
					m.err = err
					return m, nil
				}
			}
			m.answer, m.done = answer, true
			return m, tea.Quit
		case "ctrl+c", "esc":
			m.done = true
			return m, tea.Quit
		}
		m.err = nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m textModel) View() string {
	if m.done {
		return ""
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render(m.prompt) + "\n")
	b.WriteString(m.input.View())
	if m.err != nil {
		b.WriteString("\n" + errStyle.Render(m.err.Error()))
	}
	return b.String() + "\n"
}

// askText runs one free-text prompt. validate may be nil.
func (w *wizard) askText(prompt, def string, validate func(string) error) string {
	in := textinput.New()
	in.Placeholder = def
	in.Prompt = "› "
	in.PromptStyle = selectedStyle
	in.Focus()

	final, err := tea.NewProgram(
		textModel{input: in, prompt: prompt, def: def, validate: validate},
		tea.WithOutput(w.out),
	).Run()
	if err != nil {
		return def
	}
	answer := final.(textModel).answer
	if answer == "" {
		answer = def
	}
	w.answered(prompt, answer)
	return answer
}

// answered reprints the question and its answer, so the transcript still
// reads as a record once the prompt has erased itself.
func (w *wizard) answered(prompt, answer string) {
	fmt.Fprintf(w.out, "%s %s\n", helpStyle.Render(strings.TrimSuffix(prompt, "?")+":"), answerStyle.Render(answer))
}

// numberValidator rejects anything outside the range, with the range in
// the message rather than a bare refusal.
func numberValidator(min, max int) func(string) error {
	return func(s string) error {
		n, err := strconv.Atoi(s)
		if err != nil || n < min || n > max {
			return fmt.Errorf("a number between %d and %d", min, max)
		}
		return nil
	}
}
