package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/otiai10/marmoset"
	"github.com/otiai10/marmoset/marker"
	"github.com/slack-go/slack"
	"github.com/triax/hub/server/models"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

var (
	rsvp = template.Must(template.New("x").Parse(
		`参加:		{{len (index . "join")}}
不参加:	{{len (index . "absent")}}
未回答:	{{len (index . "unanswered")}}`))
)

func CronCheckRSVP(w http.ResponseWriter, req *http.Request) {
	m := marker.New(4000)
	render := marmoset.Render(w)
	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"marker": m.Next(), "error": err.Error()})
		return
	}
	defer client.Close()

	events := []models.Event{}
	if _, err := client.GetAll(ctx, datastore.NewQuery(models.KindEvent).
		Filter("Google.StartTime >", time.Now().Unix()*1000).
		Order("Google.StartTime").Limit(1), &events); err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"marker": m.Next(), "error": err.Error()})
		return
	}

	if len(events) == 0 {
		render.JSON(http.StatusNotFound, marmoset.P{"marker": m.Next(), "events": events})
		return
	}

	recent := events[0]
	participations := map[string]models.Participation{}
	if err := json.NewDecoder(strings.NewReader(recent.ParticipationsJSONString)).Decode(&participations); err != nil {
		render.JSON(http.StatusNotFound, marmoset.P{"marker": m.Next(), "events": events, "error": err.Error()})
		return
	}

	members := []models.Member{}
	if _, err := client.GetAll(ctx, datastore.NewQuery(models.KindMember).
		Filter("Slack.Deleted =", false), &members); err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	x := map[string][]models.Member{
		"join":       {},
		"absent":     {},
		"unanswered": {},
	}

	for _, member := range members {
		if p, ok := participations[member.Slack.ID]; ok {
			switch p.Type {
			case models.PTJoin, models.PTJoinLate, models.PTLeaveEarly:
				x["join"] = append(x["join"], member)
			case models.PTAbsent:
				x["absent"] = append(x["absent"], member)
			default:
				x["join"] = append(x["join"], member)
			}
		} else {
			x["unanswered"] = append(x["unanswered"], member)
		}
	}

	channel := req.URL.Query().Get("channel")
	if channel == "" {
		channel = "random"
	}
	link := ":football: https://hub.triax.football"
	text := bytes.NewBuffer(nil)
	if err := rsvp.Execute(text, x); err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"marker": m.Next(), "error": err.Error()})
		return
	}

	token := os.Getenv("SLACK_BOT_USER_OAUTH_TOKEN")
	api := slack.New(token)
	_, ts, err := api.PostMessage("#"+channel, slack.MsgOptionBlocks(
		slack.NewHeaderBlock(slack.NewTextBlockObject(slack.PlainTextType, recent.Google.Title, false, false)),
		slack.NewSectionBlock(slack.NewTextBlockObject(slack.PlainTextType, text.String(), false, false), nil, nil),
		slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType, link, false, false), nil, nil),
	))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"marker": m.Next(), "error": err.Error()})
		return
	}

	// 未回答一覧をスレッドに投稿. 将来的にはメンションにする.
	unanswered := []string{}
	for _, m := range x["unanswered"] {
		if !m.Slack.Deleted && !m.Slack.IsBot && !m.Slack.IsAppUser {
			unanswered = append(unanswered, m.Slack.RealName)
		}
	}
	if _, _, err := api.PostMessage("#"+channel,
		slack.MsgOptionText("*未回答*\n"+strings.Join(unanswered, "\n"), false),
		slack.MsgOptionTS(ts),
	); err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"marker": m.Next(), "error": err.Error()})
		return
	}

	render.JSON(http.StatusOK, marmoset.P{"event": recent, "summary": map[string]int{
		string(models.PTJoin):       len(x[string(models.PTJoin)]),
		string(models.PTAbsent):     len(x[string(models.PTAbsent)]),
		string(models.PTUnanswered): len(x[string(models.PTUnanswered)]),
	}, "timestamp": ts})
}

func CronFetchGoogleEvents(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	jsonstr := os.Getenv("GOOGLE_SERVICE_ACCOUNT_JSON")
	opt := option.WithCredentialsJSON([]byte(jsonstr))
	service, err := calendar.NewService(ctx, opt)
	if err != nil {
		fmt.Println("[ERROR]", 7001, err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err.Error())
		return
	}
	id := os.Getenv("GOOGLE_CALENDAR_ID")

	t := time.Now().Format(time.RFC3339)
	events, err := service.Events.List(id).
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(t).
		MaxResults(20).
		OrderBy("startTime").
		Do()
	if err != nil {
		fmt.Println("[ERROR]", 7002, err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err.Error())
		return
	}

	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		fmt.Println("[ERROR]", 7003, err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err.Error())
		return
	}
	defer client.Close()

	if _, err := client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		for _, item := range events.Items {
			ev := models.Event{}
			key := datastore.NameKey(models.KindEvent, item.Id, nil)
			if err := tx.Get(key, &ev); err != nil {
				fmt.Printf("[DEBUG] NEW EVENT: %+v\n", item)
			}
			ev.Google = models.CreateEventFromCalendarAPI(item)
			if _, err := tx.Put(key, &ev); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		fmt.Println("[ERROR]", 7005, err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err.Error())
		return
	}

	marmoset.Render(w).JSON(http.StatusOK, marmoset.P{
		"message": "ok",
		"count":   len(events.Items),
	})
}

func CronFetchSlackMembers(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	endpoint := "https://slack.com/api/users.list"

	slackreq, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		fmt.Println("[ERROR]", 4001, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	token := os.Getenv("SLACK_BOT_USER_OAUTH_TOKEN")
	slackreq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	res, err := http.DefaultClient.Do(slackreq)
	if err != nil || res.StatusCode != http.StatusOK {
		fmt.Println("[ERROR]", 4002, err, res.StatusCode, res.Status)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	slackres := new(models.SlackMembersResponse)
	if err := json.NewDecoder(res.Body).Decode(slackres); err != nil {
		fmt.Println("[ERROR]", 4003, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !slackres.OK {
		fmt.Println("[ERROR]", 4004, slackres.Error)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		fmt.Println("[ERROR]", 4005, slackres.Error)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer client.Close()

	keys := []*datastore.Key{}
	members := []models.Member{}
	for _, m := range slackres.Members {
		if m.IsBot || m.IsAppUser {
			continue
		}
		keys = append(keys, datastore.NameKey("Member", m.ID, nil))
		members = append(members, models.Member{Slack: m})
	}

	if _, err := client.PutMulti(ctx, keys, members); err != nil {
		fmt.Println("[ERROR]", 4006, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	marmoset.RenderJSON(w, http.StatusOK, marmoset.P{
		"message": "ok",
		"count":   len(members),
	})
}

// Cronではないが
// Eventは、`participations_json_str` を含むので、
// .Google以下だけPUTする必要がある。
// func SyncCalendarEvetns(w http.ResponseWriter, req *http.Request) {
// 	if req.Header.Get("X-Hub-Verifier") != os.Getenv("GAS_ACCESS_VERIFIER") {
// 		w.WriteHeader(http.StatusForbidden)
// 		return
// 	}
// 	payload := struct {
// 		Events []models.GoogleEvent `json:"events"`
// 	}{}
// 	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
// 		fmt.Println("[ERROR]", 6001, err.Error())
// 		w.WriteHeader(http.StatusInternalServerError)
// 		fmt.Fprint(w, err.Error())
// 		return
// 	}

// 	ctx := req.Context()
// 	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
// 	if err != nil {
// 		fmt.Println("[ERROR]", 6002, err.Error())
// 		w.WriteHeader(http.StatusInternalServerError)
// 		fmt.Fprint(w, err.Error())
// 		return
// 	}
// 	defer client.Close()

// 	if _, err := client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
// 		for _, ggl := range payload.Events {
// 			ev := models.Event{}
// 			key := datastore.NameKey(models.KindEvent, ggl.ID, nil)
// 			if err := tx.Get(key, &ev); err != nil {
// 				fmt.Printf("[DEBUG] NEW EVENT: %+v\n", ggl.Title)
// 			}
// 			ev.Google = ggl // Merge with existing "ev"
// 			if _, err := tx.Put(key, &ev); err != nil {
// 				return err
// 			}
// 		}
// 		return nil
// 	}); err != nil {
// 		fmt.Println("[ERROR]", 6005, err.Error())
// 		w.WriteHeader(http.StatusInternalServerError)
// 		fmt.Fprint(w, err.Error())
// 		return
// 	}

// 	fmt.Printf("%+v\n", payload.Events)
// 	w.WriteHeader(http.StatusOK)
// }
