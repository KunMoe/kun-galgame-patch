package ziprange

import (
	clist "container/list"
	"sync"
)

type directory struct {
	size  int64
	segs  []segment
	bytes int64
}

type dirCache struct {
	mu     sync.Mutex
	budget int64
	used   int64
	order  *clist.List
	byKey  map[string]*clist.Element
}

type dirEntry struct {
	key string
	dir *directory
}

func newDirCache(budget int64) *dirCache {
	return &dirCache{budget: budget, order: clist.New(), byKey: make(map[string]*clist.Element)}
}

func (c *dirCache) get(key string) *directory {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.byKey[key]
	if !ok {
		return nil
	}
	c.order.MoveToFront(el)
	return el.Value.(*dirEntry).dir
}

func (c *dirCache) put(key string, d *directory) {
	if d.bytes > c.budget {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.byKey[key]; ok {
		c.used -= el.Value.(*dirEntry).dir.bytes
		c.order.Remove(el)
	}
	c.byKey[key] = c.order.PushFront(&dirEntry{key: key, dir: d})
	c.used += d.bytes
	for c.used > c.budget {
		last := c.order.Back()
		e := last.Value.(*dirEntry)
		c.order.Remove(last)
		delete(c.byKey, e.key)
		c.used -= e.dir.bytes
	}
}
