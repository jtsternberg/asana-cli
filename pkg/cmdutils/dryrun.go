package cmdutils

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/timwehrle/asana/pkg/iostreams"
)

// PrintDryRun renders the request a command would have sent and reports that
// nothing was sent.
//
// It exists mainly for rich-text descriptions: --markdown-notes generates
// html_notes for you, and the only other way to see what it generated is to
// create the task and read it back. Printing the payload lets a caller check
// the markup — and rehearse a command against a real workspace — without
// writing anything.
func PrintDryRun(io *iostreams.IOStreams, endpoint string, request any) error {
	// Escaping < and > would render the html_notes value unreadable, which is
	// the main thing anyone runs a dry run to look at.
	var payload bytes.Buffer
	encoder := json.NewEncoder(&payload)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(request); err != nil {
		return fmt.Errorf("failed to render the dry-run payload: %w", err)
	}

	cs := io.ColorScheme()
	io.Printf("%s Dry run: no request was made\n", cs.WarningIcon)
	io.Printf("  %s %s\n", cs.Gray("Would send:"), endpoint)
	io.Printf("%s", payload.String())
	return nil
}
