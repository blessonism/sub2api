package dto

import "testing"

func TestNormalizeCustomMenuOpenMode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty defaults to embed", input: "", want: CustomMenuOpenModeEmbed},
		{name: "whitespace defaults to embed", input: "  ", want: CustomMenuOpenModeEmbed},
		{name: "embed", input: "embed", want: CustomMenuOpenModeEmbed},
		{name: "external", input: "external", want: CustomMenuOpenModeExternal},
		{name: "invalid", input: "popup", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := NormalizeCustomMenuOpenMode(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidateCustomMenuOpenModeWithURL(t *testing.T) {
	t.Parallel()

	if err := ValidateCustomMenuOpenModeWithURL(CustomMenuOpenModeExternal, "https://canvas.example"); err != nil {
		t.Fatalf("https external should be allowed: %v", err)
	}
	if err := ValidateCustomMenuOpenModeWithURL(CustomMenuOpenModeEmbed, "md:help"); err != nil {
		t.Fatalf("markdown embed should be allowed: %v", err)
	}
	if err := ValidateCustomMenuOpenModeWithURL(CustomMenuOpenModeExternal, "md:help"); err == nil {
		t.Fatal("expected error for external markdown URL")
	}
}
