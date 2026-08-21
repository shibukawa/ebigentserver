package cli

import (
	"strings"
	"testing"
)

// The reason for an option is the useful half of it, so the delegate has
// to render it rather than only the label. This is what the plain-text
// list already did, and the point of the TUI is to look better, not to
// say less.
func TestChoiceRendersEveryLabelAndReason(t *testing.T) {
	options := []string{"one player", "two players (realtime required)"}
	help := map[string]string{
		"one player":                      "one seat; no link",
		"two players (realtime required)": "reaching each other in one hop",
	}
	view := choiceModel{list: newChoiceList("How is it played?", options, 0, help)}.View()
	for _, want := range append(options, "one seat; no link", "reaching each other in one hop", "How is it played?") {
		if !strings.Contains(view, want) {
			t.Errorf("the rendered question is missing %q:\n%s", want, view)
		}
	}
}

// The default has to be visibly the default, or a wizard that accepts on
// enter is guessing on the reader's behalf without saying so.
func TestChoiceMarksTheDefault(t *testing.T) {
	options := []string{"a", "b", "c"}
	first := choiceModel{list: newChoiceList("q", options, 0, map[string]string{})}.View()
	third := choiceModel{list: newChoiceList("q", options, 2, map[string]string{})}.View()
	if first == third {
		t.Fatal("selecting a different default changed nothing in the render")
	}
	if !strings.Contains(first, "›") || !strings.Contains(third, "›") {
		t.Error("the cursor should mark the selected option")
	}
}

// A finished prompt erases itself, since the answer is reprinted after.
func TestFinishedPromptRendersNothing(t *testing.T) {
	m := choiceModel{list: newChoiceList("q", []string{"a"}, 0, map[string]string{}), quitting: true}
	if got := m.View(); got != "" {
		t.Errorf("a quitting prompt rendered %q", got)
	}
	if got := (textModel{done: true}).View(); got != "" {
		t.Errorf("a finished text prompt rendered %q", got)
	}
}

// The range belongs in the message: a bare refusal makes the reader guess.
func TestNumberValidatorNamesTheRange(t *testing.T) {
	v := numberValidator(2, 8)
	if err := v("4"); err != nil {
		t.Errorf("4 is in range: %v", err)
	}
	for _, bad := range []string{"1", "9", "many", ""} {
		err := v(bad)
		if err == nil {
			t.Errorf("%q should be rejected", bad)
			continue
		}
		if !strings.Contains(err.Error(), "between 2 and 8") {
			t.Errorf("error for %q does not name the range: %v", bad, err)
		}
	}
}
