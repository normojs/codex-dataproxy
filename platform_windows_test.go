//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestNormalizeWindowsSandboxReplacesElevatedMode(t *testing.T) {
	input := "model = \"gpt-5.5\"\r\n\r\n[windows]\r\nsandbox = \"elevated\"\r\n\r\n[desktop]\r\nsansFontSize = 14\r\n"

	got := normalizeWindowsSandbox(input)
	if strings.Contains(got, `sandbox = "elevated"`) {
		t.Fatalf("normalizeWindowsSandbox() should remove elevated sandbox mode:\n%s", got)
	}
	if !strings.Contains(got, "[windows]\r\nsandbox = \"unelevated\"") {
		t.Fatalf("normalizeWindowsSandbox() should keep [windows] with unelevated sandbox:\n%s", got)
	}
	if !strings.Contains(got, "[desktop]\r\nsansFontSize = 14") {
		t.Fatalf("normalizeWindowsSandbox() should preserve later sections:\n%s", got)
	}
}

func TestNormalizeWindowsSandboxPreservesOtherWindowsSettings(t *testing.T) {
	input := "[windows]\nsandbox = \"elevated\"\nother = \"kept\"\n\n[desktop]\ncodeFontSize = 13\n"

	got := normalizeWindowsSandbox(input)
	for _, want := range []string{"[windows]", `sandbox = "unelevated"`, `other = "kept"`, "[desktop]"} {
		if !strings.Contains(got, want) {
			t.Fatalf("normalizeWindowsSandbox() missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, `sandbox = "elevated"`) {
		t.Fatalf("normalizeWindowsSandbox() should remove elevated sandbox mode:\n%s", got)
	}
}

func TestNormalizeWindowsSandboxAddsMissingWindowsSection(t *testing.T) {
	input := "model = \"gpt-5.5\"\n"

	got := normalizeWindowsSandbox(input)
	if !strings.Contains(got, "[windows]\nsandbox = \"unelevated\"") {
		t.Fatalf("normalizeWindowsSandbox() should add unelevated Windows sandbox:\n%s", got)
	}
}
