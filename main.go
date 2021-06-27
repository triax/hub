package main

import (
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/otiai10/appyaml"
	"github.com/otiai10/marmoset"
	"github.com/triax/hub/controllers"
	"github.com/triax/hub/filters"
)

func init() {
	if os.Getenv("GAE_APPLICATION") == "" {
		if _, err := appyaml.Load("app.yaml"); err != nil {
			panic(err)
		}
	}
}

func main() {

	tpl, err := template.ParseGlob("templates/*.html")
	if err != nil {
		panic(err)
	}
	marmoset.UseTemplate(tpl)

	frontend := marmoset.NewRouter()
	frontend.GET("/", controllers.Index)
	frontend.GET("/members", controllers.Members)
	frontend.Apply(new(filters.AuthFilter))

	crontask := marmoset.NewRouter()
	crontask.GET("/tasks/fetch-slack-members", controllers.CronFetchSlackMembers)

	r := marmoset.NewRouter()
	r.Subrouter(frontend)
	r.Subrouter(crontask)
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
