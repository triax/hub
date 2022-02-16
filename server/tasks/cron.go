package tasks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/otiai10/marmoset"
	"github.com/slack-go/slack"
	"github.com/triax/hub/server/models"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

var (
	appbase  = os.Getenv("APP_BASE_URL")
	helplink = os.Getenv("HELP_PAGE_URL")

	rsvp = template.Must(template.New("x").Parse(
		`参加:		{{len (index . "join")}}
不参加:	{{len (index . "absent")}}
未回答:	{{len (index . "unanswered")}}`))
)

func CronCheckRSVP(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		log.Println("[ERROR]", 4001, err.Error())
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	events := []models.Event{}
	if _, err := client.GetAll(ctx, datastore.NewQuery(models.KindEvent).
		Filter("Google.StartTime >", time.Now().Unix()*1000).
		Order("Google.StartTime").Limit(1), &events); err != nil {
		log.Println("[ERROR]", 4002, err.Error())
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	if len(events) == 0 {
		log.Println("[ERROR]", 4003, err.Error())
		render.JSON(http.StatusNotFound, marmoset.P{"events": events})
		return
	}

	recent := events[0]
	participations := map[string]models.Participation{}
	if err := json.NewDecoder(strings.NewReader(recent.ParticipationsJSONString)).Decode(&participations); err != nil {
		log.Println("[ERROR]", 4004, err.Error())
		render.JSON(http.StatusNotFound, marmoset.P{"events": events, "error": err.Error()})
		return
	}

	members := []models.Member{}
	if _, err := client.GetAll(ctx, datastore.NewQuery(models.KindMember).
		Filter("Slack.Deleted =", false), &members); err != nil && !models.IsFiledMismatch(err) {
		log.Println("[ERROR]", 4005, err.Error())
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	x := map[string][]models.Member{
		"join":       {},
		"absent":     {},
		"unanswered": {},
	}

	for _, member := range members {
		if member.Status == models.MSLimited || member.Status == models.MSInactive {
			// 練習外部員や、休眠部員には、未回答メンションを送らなくてよい
			continue
		}
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

	if req.URL.Query().Get("dry") != "" {
		render.JSON(200, x)
		return
	}

	channel := req.URL.Query().Get("channel")
	if channel == "" {
		channel = "random"
	}
	link := "<" + appbase + "|:football: :football: :football: " + appbase + ">"
	text := bytes.NewBuffer(nil)
	if err := rsvp.Execute(text, x); err != nil {
		log.Println("[ERROR]", 4006, err.Error())
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
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
		log.Println("[ERROR]", 4007, err.Error())
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	reminder := buildRSVPReminderMessage(recent.Google.Title, x["unanswered"])
	if _, _, err := api.PostMessage("#"+channel, reminder, slack.MsgOptionTS(ts)); err != nil {
		log.Println("[ERROR]", 4008, err.Error())
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	render.JSON(http.StatusOK, marmoset.P{"event": recent, "summary": map[string]int{
		string(models.PTJoin):       len(x[string(models.PTJoin)]),
		string(models.PTAbsent):     len(x[string(models.PTAbsent)]),
		string(models.PTUnanswered): len(x[string(models.PTUnanswered)]),
	}, "timestamp": ts})
}

func buildRSVPReminderMessage(title string, unanswers []models.Member) slack.MsgOption {
	mentions := []string{}
	for _, m := range unanswers {
		if !m.Slack.Deleted && !m.Slack.IsBot && !m.Slack.IsAppUser {
			mentions = append(mentions, fmt.Sprintf("<@%s>", m.Slack.ID))
		}
	}
	return slack.MsgOptionBlocks(
		slack.NewHeaderBlock(slack.NewTextBlockObject(slack.PlainTextType, "出欠未回答の皆さまへ", false, false)),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, "下記のリンクから練習や試合の出欠回答ができます。伝助より使いやすいと思うので、サクッと回答お願いします。\n*<"+appbase+"|【Triax Team Hub】>*", false, false),
			nil, slack.NewAccessory(slack.NewImageBlockElement("https://avatars.slack-edge.com/2021-08-16/2369588425687_e490e60131c70bf52eee_192.png", "Triax Team Hub")),
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, "使い方やログイン方法が分からない場合は、下記のリンクで詳しい解説があるので、ご参考ください。\n*<"+helplink+"|【hubの使い方】>*", false, false),
			nil, slack.NewAccessory(slack.NewImageBlockElement("https://drive.google.com/uc?id=1OdopRR5hbOCoAftxfc4enVPbDHF37jcj", "How to use")),
		),
		slack.NewDividerBlock(),
		slack.NewContextBlock("", slack.NewTextBlockObject(slack.PlainTextType, fmt.Sprintf("直近の「%s」へ出欠回答していない人", title), false, false)),
		slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType, strings.Join(mentions, " "), false, false), nil, nil),
	)
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

	if req.URL.Query().Get("dry") != "" {
		marmoset.RenderJSON(w, 200, events)
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
	token := os.Getenv("SLACK_BOT_USER_OAUTH_TOKEN")
	api := slack.New(token)

	team, err := api.GetTeamInfoContext(ctx)
	if err != nil {
		fmt.Println("[ERROR]", 4001, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	users, err := api.GetUsersContext(ctx)
	if err != nil {
		fmt.Println("[ERROR]", 4002, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if req.URL.Query().Get("dry") != "" {
		marmoset.RenderJSON(w, http.StatusOK, users)
		return
	}

	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		fmt.Println("[ERROR]", 4005, err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer client.Close()

	count := 0
	newjoiner := []models.Member{}
	for _, u := range users {

		if u.IsBot || u.IsAppUser {
			continue
		}

		key := datastore.NameKey(models.KindMember, u.ID, nil)
		member := models.Member{}

		if _, err := client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
			if err := tx.Get(key, &member); err != nil {
				if _, ok := err.(*datastore.ErrFieldMismatch); !ok {
					fmt.Printf("[DEBUG] NEW MEMBER: %+v\n", member)
					newjoiner = append(newjoiner, member)
				}
			}

			// いずれにしても、存在しているSlack上の情報で上書き
			member.Slack = u
			member.Team = *team

			if _, err := tx.Put(key, &member); err != nil {
				return err
			}
			count++
			return nil
		}); err != nil {
			fmt.Println("[ERROR]", 4005, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	marmoset.RenderJSON(w, http.StatusOK, marmoset.P{
		"message": "ok",
		"new":     newjoiner,
		"count":   count,
	})
}
