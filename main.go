package main

import (
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/otiai10/marmoset"
	"github.com/triax/hub/server/controllers"
	"github.com/triax/hub/server/filters"
)

func main() {

	tpl := template.Must(template.ParseGlob("client/dest/*.html"))
	marmoset.UseTemplate(tpl)

	root := marmoset.NewRouter()

	authrequired := marmoset.NewRouter()
	authrequired.GET("/", controllers.Top)
	authrequired.Apply(new(filters.AuthFilter))
	root.Subrouter(authrequired)

	unauthorized := marmoset.NewRouter()
	unauthorized.GET("/login", controllers.Login)
	root.Subrouter(unauthorized)

	root.NotFound(controllers.NotFound)

	// GAEにデプロイされた場合、Staticのレンダリングは、app.yamlに任せる
	if os.Getenv("GAE_APPLICATION") == "" {
		root.Static("/_next", "client/dest/_next")
	}

	http.Handle("/", root)

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
