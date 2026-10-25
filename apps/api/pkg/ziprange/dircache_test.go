package ziprange

import "testing"

func TestDirCacheEvictsTheLeastRecentlyUsed(t *testing.T) {
	c := newDirCache(10)
	dir := func(n int64) *directory { return &directory{bytes: n} }
	c.put("a", dir(4))
	c.put("b", dir(4))
	if c.get("a") == nil {
		t.Fatal("a evicted early")
	}
	c.put("c", dir(4))
	if c.get("b") != nil {
		t.Fatal("b should be the one evicted")
	}
	if c.get("a") == nil || c.get("c") == nil {
		t.Fatal("a and c should survive")
	}
	c.put("huge", dir(11))
	if c.get("huge") != nil || c.used != 8 {
		t.Fatalf("oversized entry cached, used = %d", c.used)
	}
	c.put("a", dir(6))
	if c.used != 10 || c.get("a").bytes != 6 {
		t.Fatalf("replacing a: used = %d", c.used)
	}
}
