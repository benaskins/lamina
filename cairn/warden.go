package cairn

import (
	"fmt"
	"strings"
)

// WardenRulesPrompt returns the static Cairn rules text for the Warden system prompt.
func WardenRulesPrompt() string {
	return NewEngine().RulesText()
}

// WardenStatePrompt assembles the dynamic character state section.
func WardenStatePrompt(char *Sheet, location string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Current state:\n")
	fmt.Fprintf(&b, "%s\n", formatSheet(char))
	if location != "" {
		fmt.Fprintf(&b, "Location: %s\n", location)
	}
	return b.String()
}
