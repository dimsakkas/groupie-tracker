package handlers

import (
	"fmt"
	"net/http"
)

func (srv *Server) HomeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Groupie Tracker - %d artists loaded", len(srv.Store.ArtistsByID))
}
