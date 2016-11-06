package main

import (
	"net/http"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"

	"github.com/avelino/cover.run/views"
)

func main() {
	n := negroni.Classic()
	r := mux.NewRouter()
	r.HandleFunc("/", views.HandleHome).Methods("GET")
	r.HandleFunc("/about", views.HandleAbout).Methods("GET")
	r.PathPrefix("/assets").Handler(
		http.StripPrefix("/assets", http.FileServer(http.Dir("./assets/"))))
	r.HandleFunc("/_/{repo:.*}", views.HandleDocker)
	r.HandleFunc("/_cache/{repo:.*}", views.HandleCache)
	r.HandleFunc("/_badge/{repo:.*}", views.HandleBadge)
	r.HandleFunc("/{repo:.*}", views.HandleRepo)

	n.UseHandler(r)
	n.Run(":3000")
}
