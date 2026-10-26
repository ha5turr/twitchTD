package main

import (
	"slices"
	"testing"
)

func TestEmoteChannelsKeepOrderAndIDs(t *testing.T) {
	// Numeric IDs need no lookup; duplicates and blanks drop out.
	got := emoteChannels("458089141", []string{" 123 ", "", "458089141", "@456"})
	if want := []string{"458089141", "123"}; !slices.Equal(got[:2], want) {
		t.Fatalf("channels in config order: %v", got)
	}
	if !allDigits("458089141") || allDigits("ha5turr") || allDigits("") {
		t.Fatal("an ID is all digits")
	}
}
