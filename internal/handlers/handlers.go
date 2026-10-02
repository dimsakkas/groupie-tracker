package handlers

import (
	"groupie-tracker/internal/search"
	"groupie-tracker/internal/store"
)

type Server struct {
	Store *store.Store
	Trie  *search.Trie
}
