//go:build unit

package service

import (
	"reflect"
	"testing"
)

func TestParseCustomMenuItemURLsSkipsExternalAndMarkdown(t *testing.T) {
	t.Parallel()

	raw := `[
		{"url":"https://embed.example/page","open_mode":"embed"},
		{"url":"https://jump.example/app","open_mode":"external"},
		{"url":"https://legacy.example/help"},
		{"url":"md:docs","open_mode":"embed"}
	]`

	got := parseCustomMenuItemURLs(raw)
	want := []string{"https://embed.example/page", "https://legacy.example/help"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
