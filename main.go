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

	"github.com/go-chi/chi/v5"
)

func init() {
	if os.Getenv("GAE_APPLICATION") == "" {
		if _, err := appyaml.Load("secrets.local.yaml"); err != nil {
			panic(err)
		}
	}
}

func main() {

	marmoset.LoadViews("client/dest")

	r := chi.NewRouter()

	// API
	v1 := chi.NewRouter()
	auth := &filters.Auth{API: true, LocalDev: os.Getenv("GAE_APPLICATION") == ""}
	v1.Use(auth.Handle)
	v1.Get("/members/{id}", api.GetMember)
	v1.Post("/members/{id}/props", api.UpdateMemberProps)
	v1.Get("/members", api.ListMembers)
	v1.Get("/myself", api.GetCurrentUser)
	v1.Get("/events/{id}", api.GetEvent)
	v1.Post("/events/{id}/delete", api.DeleteEvent)
	v1.Post("/events/answer", api.AnswerEvent)
	v1.Get("/events", api.ListEvents)
	r.Mount("/api/1", v1)

	// Unauthorized pages
	r.Get("/login", controllers.Login)
	r.Post("/login", controllers.Login)
	r.Post("/logout", controllers.Logout)
	r.Get("/auth/start", controllers.AuthStart)
	r.Get("/auth/callback", controllers.AuthCallback)

	// Pages
	page := &filters.Auth{API: false, LocalDev: os.Getenv("GAE_APPLICATION") == ""}
	r.With(page.Handle).Get("/", controllers.Top)
	r.With(page.Handle).Get("/members", controllers.Members)
	r.With(page.Handle).Get("/members/{id}", controllers.Member)
	r.With(page.Handle).Get("/events", controllers.Events)
	r.With(page.Handle).Get("/events/{id}", controllers.Event)

	// Cron or Gas
	cron := chi.NewRouter()
	cron.Get("/fetch-slack-members", controllers.CronFetchSlackMembers)
	cron.Get("/fetch-calendar-events", controllers.CronFetchGoogleEvents)
	cron.Get("/check-rsvp", controllers.CronCheckRSVP)
	r.Mount("/tasks", cron)

	r.NotFound(controllers.NotFound)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}
	log.Printf("Listening on port %s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}
