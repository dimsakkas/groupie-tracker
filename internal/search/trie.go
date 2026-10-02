package search

import (
	"groupie-tracker/internal/models"
	"slices"
	"strings"
)

// TrieNode is one character in the trie. entries is populated only when
// this node is the end of a fully-inserted token (len(entries) > 0 tells
// you that on its own -- no separate isEnd flag needed).
type TrieNode struct {
	children map[rune]*TrieNode
	entries  []models.SearchEntry
}

func newTrieNode() *TrieNode {
	return &TrieNode{
		children: make(map[rune]*TrieNode),
	}
}

// Trie wraps the root TrieNode so callers never touch *TrieNode directly.
type Trie struct {
	root *TrieNode
}

func NewTrie() *Trie {
	return &Trie{root: newTrieNode()}
}

// Insert plants one token character-by-character into the trie and attaches
// entry to the node where the token ends. Case-insensitive by design: every
// token is lowercased before insertion, and Search lowercases its prefix
// the same way, so lookups always match regardless of how the user types.
func (t *Trie) Insert(token string, entry models.SearchEntry) {
	token = strings.ToLower(token)
	node := t.root
	for _, ch := range token {
		child, ok := node.children[ch]
		if !ok {
			child = newTrieNode()
			node.children[ch] = child
		}
		node = child
	}
	node.entries = append(node.entries, entry)
}

// InsertMultiToken splits text on '-', '_' and whitespace and inserts entry
// under each resulting token. A multi-word value like "north_carolina-usa"
// becomes reachable from "north", "carolina" AND "usa" individually -- all
// three point back to the exact same entry.
func (t *Trie) InsertMultiToken(text string, entry models.SearchEntry) {
	tokens := strings.FieldsFunc(text, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	for _, token := range tokens {
		t.Insert(token, entry)
	}
}

// Search walks to the node matching prefix and collects up to limit
// entries: the node's own entries first (exact-token matches), then its
// descendants in alphabetical order (longer completions). limit is a hard
// cap -- collector never appends past it, unlike a naive "check before,
// append everything" approach that can silently overshoot. seen dedups: a
// multi-token insert (see InsertMultiToken) attaches the SAME entry at
// several different leaf nodes, so a broad prefix that reaches more than
// one of them would otherwise return that entry more than once.
func (t *Trie) Search(prefix string, limit int) []models.SearchEntry {
	prefix = strings.ToLower(prefix)
	node := t.root
	for _, ch := range prefix {
		child, ok := node.children[ch]
		if !ok {
			return nil
		}
		node = child
	}

	var results []models.SearchEntry
	seen := make(map[models.SearchEntry]bool)
	collector(node, limit, &results, seen)
	return results
}

func collector(node *TrieNode, limit int, results *[]models.SearchEntry, seen map[models.SearchEntry]bool) {
	if len(*results) >= limit {
		return
	}

	for _, entry := range node.entries {
		if len(*results) >= limit {
			return
		}
		if seen[entry] {
			continue
		}
		seen[entry] = true
		*results = append(*results, entry)
	}

	keys := make([]rune, 0, len(node.children))
	for ch := range node.children {
		keys = append(keys, ch)
	}
	slices.Sort(keys)

	for _, ch := range keys {
		collector(node.children[ch], limit, results, seen)
	}
}
