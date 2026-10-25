package archive

import "strings"

const (
	outlineAbove = 2000
	outlineDepth = 2
)

// The web reads a non-zero Items as "children not shipped" and refetches
// with ?full=1 (archiveTree.ts isUnloaded).
func (l *Listing) Outline() {
	if len(l.Entries) <= outlineAbove {
		return
	}
	out := make(FileEntries, 0, 64)
	seen := make(map[string]struct{})
	cut := make(map[string]int)
	child := make(map[string]struct{})
	for _, e := range l.Entries {
		dir := strings.HasSuffix(e.Path, "/")
		segs := strings.Split(strings.TrimSuffix(e.Path, "/"), "/")
		ancestors := len(segs)
		if !dir {
			ancestors--
		}
		for d := 1; d <= min(ancestors, outlineDepth); d++ {
			p := strings.Join(segs[:d], "/") + "/"
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			if d == outlineDepth {
				cut[p] = len(out)
			}
			out = append(out, FileEntry{Path: p})
		}
		if len(segs) <= outlineDepth {
			if !dir {
				out = append(out, e)
			}
			continue
		}
		at := &out[cut[strings.Join(segs[:outlineDepth], "/")+"/"]]
		key := strings.Join(segs[:outlineDepth+1], "/")
		if _, ok := child[key]; !ok {
			child[key] = struct{}{}
			at.Items++
		}
		if !dir {
			at.Files++
			at.Size += e.Size
		}
	}
	l.Entries = out
	l.Truncated = true
}
