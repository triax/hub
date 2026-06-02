package main

import (
	"log"
	"net/http"
	"os"

	"github.com/otiai10/appyaml"
	"github.com/otiai10/marmoset"
	"github.com/otiai10/openaigo"
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

	// セキュリティヘッダー
	r.Use(filters.SecurityHeaders)

	// API
	v1 := chi.NewRouter()
	auth := &filters.Auth{API: true, LocalDev: os.Getenv("GAE_APPLICATION") == ""}
	v1.Use(auth.Handle)

	// 写真アップロードは 10MB まで許容（他の全エンドポイントは下の Group で 1MB）
	v1.With(filters.MaxBodySize(10<<20)).Post("/members/{id}/hp-profile/photo", api.UploadHPPhoto)

	v1.Group(func(r chi.Router) {
		r.Use(filters.MaxBodySize(1 << 20)) // 1MB
		r.Get("/members/{id}", api.GetMember)
		r.Post("/members/{id}/props", api.UpdateMemberProps)
		r.Get("/members/{id}/hp-profile", api.GetHPProfile)
		r.Put("/members/{id}/hp-profile", api.UpdateHPProfile)
		r.Get("/members", api.ListMembers)
		r.Get("/myself", api.GetCurrentUser)
		r.Get("/events/{id}", api.GetEvent)
		r.Post("/events/{id}/delete", api.DeleteEvent)
		r.Post("/events/answer", api.AnswerEvent)
		r.Get("/events", api.ListEvents)
		// Equips
		r.Post("/equips/custody", api.EquipCustodyReport)
		r.Get("/equips/{id}", api.GetEquip)
		r.Post("/equips/{id}/delete", api.DeleteEquip)
		r.Post("/equips/{id}/update", api.UpdateEquip)
		r.Post("/equips", api.CreateEquipItem)
		r.Get("/equips", api.ListEquips)
		r.Post("/numbers/{num}/assign", api.AssignPlayerNumber)
		r.Post("/numbers/{num}/deprive", api.DeprivePlayerNumber)
		r.Get("/numbers", api.GetAllNumbers)
		// TapeItem
		r.Get("/tape-items", api.ListTapeItems)
		r.Post("/tape-items", api.CreateTapeItem)
		r.Post("/tape-items/{id}/update", api.UpdateTapeItem)
		r.Post("/tape-items/{id}/delete", api.DeleteTapeItem)
		// Taping
		r.Get("/taping/menu", api.ListTapingMenuItems)
		r.Post("/taping/menu", api.CreateTapingMenuItem)
		r.Post("/taping/menu/{id}/update", api.UpdateTapingMenuItem)
		r.Post("/taping/menu/{id}/delete", api.DeleteTapingMenuItem)
		r.Get("/taping/requests", api.ListTapingRequests)
		r.Post("/taping/requests", api.SubmitTapingRequest)
		r.Get("/taping/requests/me", api.GetMyTapingRequest)
		r.Get("/taping/events", api.ListTapingEvents)
		// Applications
		r.Get("/applications", api.GetApplications)
		r.Patch("/applications/{id}", api.UpdateApplication)
	})
	r.Mount("/api/1", v1)

	// 認証不要の公開 API（外部 HP サイト向け、および公開フォーム）
	r.With(filters.MaxBodySize(1<<20)).Get("/api/1/public/members", api.ListPublicMembers)

	// 入部申請フォーム送信（認証不要）
	// NOTE: chi の Mount は /api/1/* を v1 サブルーターに転送するが、
	// /api/1/public/* のような明示パスは先に登録することで回避できる。
	// ここでは /api/1/public/applications に配置して競合を避ける。
	r.With(filters.MaxBodySize(512<<10)).Post("/api/1/public/applications", api.CreateApplication)

	// ヘルスチェック（認証不要・Datastore 非依存）
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})

	// Unauthorized pages
	smallBody := filters.MaxBodySize(1 << 20) // 1MB
	r.With(smallBody).Post("/login", controllers.Login)
	r.Get("/login", controllers.Login)
	r.With(smallBody).Post("/logout", controllers.Logout)
	r.Get("/auth/start", controllers.AuthStart)
	r.Get("/auth/callback", controllers.AuthCallback)
	r.Get("/errors", controllers.ErrorsPage)

	// Bot events
	bot := slackbot.Bot{
		VerificationToken: os.Getenv("SLACK_BOT_EVENTS_VERIFICATION_TOKEN"),
		SlackAPI:          slack.New(os.Getenv("SLACK_BOT_USER_OAUTH_TOKEN")),
		ChatGPT:           openaigo.NewClient(os.Getenv("OPENAI_API_KEY")),
	}
	r.With(smallBody).Post("/slack/events", bot.Webhook)
	r.With(smallBody).Post("/slack/shortcuts", bot.Shortcuts)
	r.With(smallBody).Post("/slack/slashcommands", bot.SlashCommands)

	// Vite SPA 静的アセット配信
	r.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.Dir("client/dest/assets"))))

	// Pages (すべて index.html を返し、TanStack Router がクライアントサイドでルーティング)
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
	r.With(page.Handle).Get("/equips/{id}/edit", controllers.EquipEdit)
	r.With(page.Handle).Get("/uniforms", controllers.Uniforms)
	r.With(page.Handle).Get("/redirect/conditioning-form", controllers.RedirectConditioningForm)
	r.With(page.Handle).Get("/taping/request", controllers.TapingRequest)
	r.With(page.Handle).Get("/taping/master", controllers.TapingMaster)
	r.With(page.Handle).Get("/taping", controllers.TapingOverview)
	r.With(page.Handle).Get("/events/{id}/taping", controllers.EventTaping)
	r.Get("/applications/onboarding", controllers.Applications)        // 公開（認証なし）
	r.With(page.Handle).Get("/applications", controllers.Applications) // 管理（認証あり）

	// Cloud Tasks (GAE cronリクエストのみ許可)
	cron := chi.NewRouter()
	if os.Getenv("GAE_APPLICATION") != "" {
		cron.Use(filters.RequireGAECron)
	}
	cron.Get("/fetch-slack-members", tasks.CronFetchSlackMembers)
	cron.Get("/fetch-calendar-events", tasks.CronFetchGoogleEvents)
	cron.Get("/check-rsvp", tasks.CronCheckRSVP)
	cron.Get("/final-call", tasks.FinalCall)
	cron.Get("/equips/remind/bring", tasks.EquipsRemindBring)
	cron.Get("/equips/remind/report", tasks.EquipsRemindReportAfterEvent)
	cron.Get("/equips/scan-unreported", tasks.EquipsScanUnreported)
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
