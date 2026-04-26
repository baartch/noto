package update

import "testing"

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"":       "v0.0.0",
		"1.2.3":  "v1.2.3",
		"v2.0.0": "v2.0.0",
	}
	for in, want := range cases {
		if got := normalize(in); got != want {
			t.Fatalf("normalize(%q)=%q want %q", in, got, want)
		}
	}
}
