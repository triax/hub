package main

import (
	"log"
	"net/http"
	"os"

	"github.com/otiai10/appyaml"
	"github.com/otiai10/marmoset"
	"github.com/slack-go/slack"
	"github.com/triax/hub/server/api"
	"github.com/triax/hub/server/controllers"
	"github.com/triax/hub/server/filters"
	"github.com/triax/hub/server/slackbot"
	"github.com/triax/hub/server/tasks"

	"github.com/go-chi/chi/v5"
)

var (
	tpl = marmoset.LoadViews("client/dest")
)

func init() {
	if os.Getenv("GAE_APPLICATION") == "" {
		if _, err := appyaml.Load("secrets.local.yaml"); err != nil {
			panic(err)
		}
	}
}

func main() {

	marmoset.UseTemplate(tpl)

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
	// Equips
	v1.Post("/equips/custody", api.EquipCustodyReport)
	v1.Get("/equips/{id}", api.GetEquip)
	v1.Post("/equips/{id}/delete", api.DeleteEquip)
	v1.Post("/equips/{id}/update", api.UpdateEquip)
	v1.Post("/equips", api.CreateEquipItem)
	v1.Get("/equips", api.ListEquips)
	r.Mount("/api/1", v1)

	// Unauthorized pages
	r.Get("/login", controllers.Login)
	r.Post("/login", controllers.Login)
	r.Post("/logout", controllers.Logout)
	r.Get("/auth/start", controllers.AuthStart)
	r.Get("/auth/callback", controllers.AuthCallback)
	r.Get("/errors", controllers.ErrorsPage)

	// Bot events
	bot := slackbot.Bot{
		VerificationToken: os.Getenv("SLACK_BOT_EVENTS_VERIFICATION_TOKEN"),
		SlackAPI:          slack.New(os.Getenv("SLACK_BOT_USER_OAUTH_TOKEN")),
	}
	r.Post("/slack/events", bot.Webhook)
	r.Post("/slack/shortcuts", bot.Shortcuts)

	// Pages
	page := &filters.Auth{API: false, LocalDev: os.Getenv("GAE_APPLICATION") == ""}
	r.With(page.Handle).Get("/", controllers.Top)
	r.With(page.Handle).Get("/members", controllers.Members)
	r.With(page.Handle).Get("/members/{id}", controllers.Member)
	r.With(page.Handle).Get("/events", controllers.Events)
	r.With(page.Handle).Get("/events/{id}", controllers.Event)
	r.With(page.Handle).Get("/equips", controllers.Equips)
	r.With(page.Handle).Get("/equips/create", controllers.EquipCreate)
	r.With(page.Handle).Get("/equips/report", controllers.EquipReport)
	r.With(page.Handle).Get("/equips/{id}", controllers.Equip)
	r.With(page.Handle).Get("/redirect/conditioning-form", controllers.RedirectConditioningForm)

	// Cloud Tasks
	cron := chi.NewRouter()
	cron.Get("/fetch-slack-members", tasks.CronFetchSlackMembers)
	cron.Get("/fetch-calendar-events", tasks.CronFetchGoogleEvents)
	cron.Get("/check-rsvp", tasks.CronCheckRSVP)
	cron.Get("/final-call", tasks.FinalCall)
	cron.Get("/equips/remind/bring", tasks.EquipsRemindBring)
	cron.Get("/equips/remind/report", tasks.EquipsRemindReport)
	cron.Get("/condition/precheck", tasks.ConditionPrecheck)   // TODO: 削除
	cron.Get("/condition/postcheck", tasks.ConditionPostcheck) // TODO: 削除
	cron.Get("/condition/form", tasks.ConditionFrom)
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
