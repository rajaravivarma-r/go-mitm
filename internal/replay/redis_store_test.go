package replay

import "testing"

func TestBuildRESPCommand(t *testing.T) {
	got := string(buildRESPCommand("GET", "alpha"))
	want := "*2\r\n$3\r\nGET\r\n$5\r\nalpha\r\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
