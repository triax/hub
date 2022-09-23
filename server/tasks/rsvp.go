package tasks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/otiai10/marmoset"
	"github.com/slack-go/slack"
	"github.com/triax/hub/server"
	"github.com/triax/hub/server/models"
)

var (
	appbase  = os.Getenv("APP_BASE_URL")
	helplink = os.Getenv("HELP_PAGE_URL")

	rsvp = template.Must(template.New("x").Parse(
		`参加:		{{len (index . "join")}}
不参加:	{{len (index . "absent")}}
未回答:	{{len (index . "unanswered")}}`))
)

func FinalCall(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	render := marmoset.Render(w, true)
	roles := strings.Split(req.URL.Query().Get("role"), ",")
	channel := req.URL.Query().Get("channel")

	events, err := models.FindEventsBetween(ctx)
	if err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err})
		return
	}
	if len(events) == 0 {
		render.JSON(http.StatusOK, marmoset.P{"events": events, "error": err})
		return
	}
	ev := events[0]

	if ev.ShouldSkipReminders() {
		render.JSON(http.StatusOK, marmoset.P{"events": events, "error": fmt.Errorf("should ignore: " + ev.Google.Title)})
		return
	}
	pats, err := ev.Participations()
	if err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": fmt.Errorf("JSON decode error: %v", err)})
		return
	}
	report := map[string][]models.Participation{}
	exps := map[string]*regexp.Regexp{}
	for _, r := range roles {
		report[r] = []models.Participation{}
		if exps[r], err = regexp.Compile("(?i)" + r); err != nil {
			render.JSON(http.StatusBadRequest, marmoset.P{"error": err})
			return
		}
	}
	for _, m := range pats {
		for r, exp := range exps {
			if exp.MatchString(m.Title) && m.Type.JoinAnyhow() {
				report[r] = append(report[r], m)
			}
		}
	}

	msg := buildFinalCallMessage(ev, roles, report)
	token := os.Getenv("SLACK_BOT_USER_OAUTH_TOKEN")
	api := slack.New(token)
	if _, _, err = api.PostMessage("#"+channel, msg); err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err})
		return
	}

	render.JSON(http.StatusOK, marmoset.P{"roles": roles, "channel": channel, "report": report})
}

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

	if recent.ShouldSkipReminders() {
		render.JSON(http.StatusNotFound, marmoset.P{"events": events})
		return
	}

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

func buildFinalCallMessage(event models.Event, roles []string, report map[string][]models.Participation) slack.MsgOption {
	blocks := []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf(
				"<%s/events/%s|%s> の出欠状況 （ポジション設定: %v）", server.HubBaseURL, event.Google.ID, event.Google.Title, roles,
			), false, false),
			nil, nil,
		),
		slack.NewDividerBlock(),
	}
	for i, role := range roles {
		if len(roles) > 1 {
			blocks = append(blocks,
				slack.NewContextBlock("",
					slack.NewTextBlockObject(slack.MarkdownType, "*"+strings.ToUpper(role)+"*", false, false),
				),
			)
		}
		if len(report[role]) == 0 {
			blocks = append(blocks, slack.NewContextBlock("",
				slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf(
					"Slackプロフィールで「役職（Title）」を「%s」や「%s」などに設定している人はいません.",
					role, strings.ToUpper(role),
				), false, false),
			))
		} else {
			names := []string{}
			for _, m := range report[role] {
				names = append(names, m.Name)
			}
			blocks = append(blocks,
				slack.NewSectionBlock(
					slack.NewTextBlockObject(slack.MarkdownType, strings.Join(names, ",  "), false, false),
					nil, nil,
				),
			)
		}
		if i+1 < len(roles) {
			blocks = append(blocks, slack.NewDividerBlock())
		}
	}
	return slack.MsgOptionBlocks(blocks...)
}
