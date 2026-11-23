package discovery

import (
	"testing"
	"time"

	"github.com/geteduroam/linux-app/internal/provider"
)

func TestDiskCache(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	c := NewCache()
	if err := c.load(); err == nil {
		t.Fatal("loading a cache that does not exist should fail")
	}
	c.Cached.Value.Seq = 5
	c.Cached.Value.Providers = provider.Providers{{ID: "test"}}
	c.LastUpdate = time.Now()
	if err := c.save(); err != nil {
		t.Fatalf("failed saving cache: %v", err)
	}

	// this is recent, so this must not do a request
	got, err := NewCache().Providers()
	if err != nil {
		t.Fatalf("failed getting providers: %v", err)
	}
	if len(*got) != 1 || (*got)[0].ID != "test" {
		t.Fatalf("providers not equal. Got: %v", *got)
	}
}
