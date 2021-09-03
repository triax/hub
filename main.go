package main

import (
	"log"
	"net/http"
	"os"

	"github.com/otiai10/appyaml"
	"github.com/otiai10/marmoset"
	"github.com/triax/hub/server/api"
	"github.com/triax/hub/server/controllers"
	"github.com/triax/hub/server/filters"
)

func init() {
	if os.Getenv("GAE_APPLICATION") == "" {
		if _, err := appyaml.Load("secrets.local.yaml"); err != nil {
			panic(err)
		}
	}
}

func main() {

	// tpl := template.Must(template.ParseGlob("client/dest/*.html"))
	// marmoset.UseTemplate(tpl)
	marmoset.LoadViews("client/dest")

	root := marmoset.NewRouter()

	// API
	authapis := marmoset.NewRouter()
	authapis.GET("/api/1/members/(?P<id>[a-zA-Z0-9]+)", api.GetMember)
	authapis.GET("/api/1/members", api.ListMembers)
	authapis.GET("/api/1/myself", api.GetCurrentUser)
	authapis.GET("/api/1/events/(?P<id>[a-zA-Z0-9]+)", api.GetEvent)
	authapis.POST("/api/1/events/answer", api.AnswerEvent)
	authapis.GET("/api/1/events", api.ListEvents)
	authapis.POST("/api/1/auth/logout", api.AuthLogout)
	authapis.Apply(&filters.AuthFilter{
		API: true, LocalDev: os.Getenv("GAE_APPLICATION") == "",
	})
	root.Subrouter(authapis)

	// Unauthorized pages
	unauthorized := marmoset.NewRouter()
	unauthorized.GET("/login", controllers.Login)
	unauthorized.GET("/auth/start", controllers.AuthStart)
	unauthorized.GET("/auth/callback", controllers.AuthCallback)
	root.Subrouter(unauthorized)

	// Pages
	authpages := marmoset.NewRouter()
	authpages.GET("/", controllers.Top)
	authpages.GET("/members", controllers.Members)
	authpages.GET("/members/(?P<id>[a-zA-Z0-9]+)", controllers.Member)
	authpages.GET("/events", controllers.Events)
	authpages.GET("/events/(?P<id>[a-zA-Z0-9]+)", controllers.Event)
	authpages.Apply(&filters.AuthFilter{
		API: false, LocalDev: os.Getenv("GAE_APPLICATION") == "",
	})
	root.Subrouter(authpages)

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
