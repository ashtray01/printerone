//go:build windows
// +build windows

package main

import (
	"fmt"
	"testing"
)

func TestAllSupportedLanguagesAreComplete(t *testing.T) {
	for _, option := range languageOptions {
		text, ok := uiTexts[option.code]
		if !ok {
			t.Fatalf("missing translations for %s", option.code)
		}
		values := []string{
			text.Title, text.Printer, text.Refresh, text.Address, text.Port,
			text.AutoStart, text.WindowsStart, text.Minimize, text.Stopped,
			text.Running, text.Save, text.Start, text.Started, text.Stop,
			text.CheckPort, text.TestPrint, text.Exit, text.Log, text.Ready,
			text.ServerStarted, text.Printers,
		}
		for index, value := range values {
			if value == "" {
				t.Fatalf("language %s has empty field %d", option.code, index)
			}
		}
		if got := fmt.Sprintf(text.Printers, 3); got == text.Printers {
			t.Fatalf("language %s printer count has no format placeholder", option.code)
		}
	}
}

func TestUnknownLanguageFallsBackToRussian(t *testing.T) {
	if got := normalizeLanguage("unknown"); got != "ru" {
		t.Fatalf("got %q", got)
	}
}

func TestSupportedLanguageList(t *testing.T) {
	want := []string{"ru", "en", "de", "es"}
	if len(languageOptions) != len(want) {
		t.Fatalf("got %d languages, want %d", len(languageOptions), len(want))
	}
	for index, code := range want {
		if languageOptions[index].code != code {
			t.Fatalf("language %d is %q, want %q", index, languageOptions[index].code, code)
		}
	}
}
