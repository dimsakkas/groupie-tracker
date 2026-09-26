package search

import (
	"groupie-tracker/internal/models"
	"slices"
	"strings"
)

type TrieNode struct {
	children map[rune]*TrieNode
	entries  []models.SearchEntry
}

func newTrieNode() *TrieNode {
	return &TrieNode{
		children: make(map[rune]*TrieNode),
	}
}

type Trie struct {
	root *TrieNode
}

func NewTrie() *Trie {
	return &Trie{root: newTrieNode()}
}

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

func (t *Trie) Search(prefix string) []models.SearchEntry {
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
	collect(node, &results)
	return results
}

func collect(node *TrieNode, results *[]models.SearchEntry) {
	*results = append(*results, node.entries...)

	keys := make([]rune, 0, len(node.children))
	for ch := range node.children {
		keys = append(keys, ch)
	}
	slices.Sort(keys)
	for _, ch := range keys {
		collect(node.children[ch], results)
	}
}
