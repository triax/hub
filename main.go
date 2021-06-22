package main

import (
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/otiai10/marmoset"
	"github.com/triax/hub/controllers"
	"github.com/triax/hub/filters"
)

func main() {

	tpl, err := template.ParseGlob("templates/*.html")
	if err != nil {
		panic(err)
	}
	marmoset.UseTemplate(tpl)

	r := marmoset.NewRouter()
	r.Apply(new(filters.AuthFilter))
	routes(r)
	http.Handle("/", r)

	// [START setting_port]
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	log.Printf("Listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func routes(r *marmoset.Router) {
	r.GET("/", controllers.Index)
}
