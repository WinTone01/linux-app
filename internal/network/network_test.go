package network

import "testing"

func TestValidateUsername(t *testing.T) {
	cases := []struct {
		c    Credentials
		user string
		ok   bool
	}{
		{Credentials{}, "", false},
		{Credentials{}, "user", true},
		{Credentials{Suffix: "@edu.nl"}, "user@edu.nl", true},
		{Credentials{Suffix: "@edu.nl"}, "user@sub.edu.nl", false},
		{Credentials{Prefix: "a-"}, "user", false},
		{Credentials{Realm: "pau.edu.tr"}, "user", false},
		{Credentials{Realm: "pau.edu.tr"}, "@pau.edu.tr", false},
		{Credentials{Realm: "pau.edu.tr"}, "user@pau.edu.tr", true},
		{Credentials{Realm: "pau.edu.tr"}, "user@posta.pau.edu.tr", true},
		{Credentials{Realm: "pau.edu.tr"}, "user@POSTA.pau.edu.tr", true},
		{Credentials{Realm: "pau.edu.tr"}, "user@notpau.edu.tr", false},
		{Credentials{Realm: "pau.edu.tr"}, "user@example.org", false},
	}
	for _, c := range cases {
		if err := c.c.ValidateUsername(c.user); (err == nil) != c.ok {
			t.Fatalf("validating %q with %+v. Got: %v, want ok: %v", c.user, c.c, err, c.ok)
		}
	}
}
