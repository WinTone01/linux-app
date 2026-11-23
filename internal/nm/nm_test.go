package nm

import (
	"reflect"
	"testing"
)

func TestGroupCiphers(t *testing.T) {
	cases := map[string][]string{
		"CCMP":     {"ccmp", "tkip"},
		"":         {"ccmp", "tkip"},
		"GCMP-256": {"gcmp-256", "ccmp", "tkip"},
	}
	for in, want := range cases {
		if got := groupCiphers(in); !reflect.DeepEqual(got, want) {
			t.Fatalf("group ciphers for %q not equal. Got: %v, want: %v", in, got, want)
		}
	}
}

func TestDomainMatch(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{"", " "}, ""},
		{[]string{"eduroam.pau.edu.tr"}, "eduroam.pau.edu.tr"},
		{[]string{"a.example.org", " b.example.org ", "a.example.org"}, "a.example.org;b.example.org"},
	}
	for _, c := range cases {
		if got := domainMatch(c.in); got != c.want {
			t.Fatalf("domain match for %v not equal. Got: %q, want: %q", c.in, got, c.want)
		}
	}
}
