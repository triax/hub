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

// func init() {
// 	if os.Getenv("GAE_APPLICATION") == "" {
// 		if _, err := appyaml.Load("app.yaml"); err != nil {
// 			panic(err)
// 		}
// 	}
// }

func main() {

	tpl := template.Must(template.ParseGlob("client/dest/*.html"))
	marmoset.UseTemplate(tpl)

	root := marmoset.NewRouter()

	// Pages
	authrequired := marmoset.NewRouter()
	authrequired.GET("/", controllers.Top)
	authrequired.Apply(new(filters.AuthFilter))
	root.Subrouter(authrequired)

	// Unauthorized pages
	unauthorized := marmoset.NewRouter()
	unauthorized.GET("/login", controllers.Login)
	root.Subrouter(unauthorized)

	// Cron or Gas
	cron := marmoset.NewRouter()
	cron.GET("/tasks/fetch-slack-members", controllers.CronFetchSlackMembers)
	cron.POST("/_gas/sync-calendar-events", controllers.SyncCalendarEvetns)
	root.Subrouter(cron)

	// 404
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
