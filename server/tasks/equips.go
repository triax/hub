package tasks

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/otiai10/marmoset"
	"github.com/slack-go/slack"
	"github.com/triax/hub/server"
	"github.com/triax/hub/server/models"
)

type (
	EquipAlloc struct {
		Event       models.Event
		OK          map[string][]models.Equip
		NG          map[string][]models.Equip
		Unnecessary []models.Equip
	}
)

func EquipsRemindBring(w http.ResponseWriter, req *http.Request) {

	ctx := req.Context()
	render := marmoset.Render(w, true)

	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		log.Println("[ERROR]", 8001, err.Error())
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	// 1) 直近24時間以内のイベントを取得
	events := []models.Event{}
	query := datastore.NewQuery(models.KindEvent).
		Filter("Google.StartTime >", time.Now().Unix()*1000).
		Filter("Google.StartTime <=", time.Now().Add(24*time.Hour).Unix()*1000).
		Order("Google.StartTime").
		Limit(1)
	if _, err := client.GetAll(ctx, query, &events); err != nil {
		log.Println("[ERROR]", 8002, err.Error())
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	// Eventが無ければ終了
	if len(events) == 0 {
		render.JSON(http.StatusOK, marmoset.P{"events": events, "message": "not found"})
		return
	}
	ev := events[0]

	if ev.ShouldSkipReminders() {
		render.JSON(http.StatusOK, marmoset.P{"events": events, "message": "not found"})
		return
	}

	// 3) 全Equipsの所持者を取得する
	equips := []models.Equip{}
	query = datastore.NewQuery(models.KindEquip)
	if ev.IsGame() {
		query.Filter("ForGame =", true)
	} else if ev.IsPractice() {
		query.Filter("ForPractice =", true)
	}
	if _, err := client.GetAll(ctx, query, &equips); err != nil && !models.IsFiledMismatch(err) {
		log.Println("[ERROR]", 8003, err.Error())
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	for i, e := range equips {
		equips[i].ID = e.Key.ID
		// 最新のHistoryだけ収集する
		query := datastore.NewQuery(models.KindCustody).Ancestor(e.Key).Order("-Timestamp").Limit(1)
		client.GetAll(ctx, query, &equips[i].History) // エラーは無視してよい
	}

	// 4) 全Equipsの所持者に対してSlackにメンションを送る
	pats, err := ev.Participations()
	if err != nil {
		log.Println("[ERROR]", 8004, err.Error())
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
	}
	alloc := summarizeEquipAllocForTheEvent(ev, equips, pats)

	api := slack.New(os.Getenv("SLACK_BOT_USER_OAUTH_TOKEN"))
	msg := buildEquipsReminderMsg(alloc)
	if _, _, err := api.PostMessageContext(ctx, "random", msg); err != nil {
		log.Println("[ERROR]", 8005, err.Error())
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
	}

	render.JSON(http.StatusOK, alloc)
}

func summarizeEquipAllocForTheEvent(event models.Event, equips []models.Equip, pats models.Participations) EquipAlloc {
	alloc := EquipAlloc{
		Event:       event,
		OK:          map[string][]models.Equip{},
		NG:          map[string][]models.Equip{},
		Unnecessary: []models.Equip{},
	}
	for _, e := range equips {
		if event.IsPractice() && !e.ForPractice {
			continue // 練習イベントだが、練習用装備ではないため、スルー
		}
		if event.IsGame() && !e.ForGame {
			continue // 試合イベントだが、試合用装備ではないため、スルー
		}
		if len(e.History) == 0 {
			log.Printf("[ERROR] 誰も管理していない: %s", e.Name)
			continue
		}
		p, ok := pats[e.History[0].MemberID]
		switch {
		case !ok: // 未回答
			fallthrough
		case p.Type == models.PTAbsent: // 欠席
			alloc.NG[e.History[0].MemberID] = append(alloc.NG[e.History[0].MemberID], e)
		default:
			alloc.OK[e.History[0].MemberID] = append(alloc.OK[e.History[0].MemberID], e)
		}
	}
	return alloc
}

func buildEquipsReminderMsg(alloc EquipAlloc) slack.MsgOption {
	blocks := []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject(
				slack.MarkdownType,
				fmt.Sprintf(
					"【前日確認】備品を持って帰ってくれている皆さまへ `%s` にて以下の備品を持ってきていただけるようお願いします :bow:",
					alloc.Event.Google.Title,
				),
				false, false,
			), nil, nil,
		),
	}
	for uid, equips := range alloc.OK {
		names := []string{}
		for _, e := range equips {
			if e.NeedsCharge() {
				names = append(names, ":electric_plug::zap: _"+e.Name+"_")
			} else {
				names = append(names, "_"+e.Name+"_")
			}
		}
		blocks = append(blocks,
			slack.NewSectionBlock(nil, []*slack.TextBlockObject{
				slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("<@%s>", uid), false, false),
				slack.NewTextBlockObject(slack.MarkdownType, strings.Join(names, "\n"), false, false),
			}, nil),
		)
	}
	return slack.MsgOptionBlocks(blocks...)
}

func EquipsRemindReport(w http.ResponseWriter, req *http.Request) {

	ctx := req.Context()
	render := marmoset.Render(w, true)

	// 本日の、指定時間に開始されているイベントを取得
	from, to, err := defineTimeRangeByRequest(time.Now(), req)
	if err != nil {
		log.Println("[ERROR]", 9001, err.Error())
		render.JSON(http.StatusBadRequest, err.Error())
		return
	}

	events, err := models.FindEventsBetween(ctx, from, to)
	if err != nil {
		log.Println("[ERROR]", 9002, err.Error())
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	// 該当イベント無し
	if len(events) == 0 {
		render.JSON(http.StatusOK, marmoset.P{"events": events, "message": "not found"})
		return
	}

	ev := events[0]

	if ev.ShouldSkipReminders() {
		render.JSON(http.StatusOK, marmoset.P{"events": events, "message": "not found"})
		return
	}

	api := slack.New(os.Getenv("SLACK_BOT_USER_OAUTH_TOKEN"))
	channel := "random"

	if _, _, err = api.PostMessageContext(ctx, channel, slack.MsgOptionBlocks(
		slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("@channel お疲れさまでした！ *%s*\n備品を持って帰って頂いた方は、以下のフォームにご回答いただくようお願いいたします :bow:\n%s/equips/report", ev.Google.Title, server.HubBaseURL()), false, false),
			nil, nil,
		),
	)); err != nil {
		log.Println("[ERROR]", 9005, err.Error())
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	render.JSON(http.StatusOK, ev)

}

func EquipsScanUnreported(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w, true)
	offsetHours, err := strconv.Atoi(req.URL.Query().Get("oh"))
	if err != nil {
		render.JSON(http.StatusBadRequest, map[string]any{"error": err})
		return
	}
	ctx := req.Context()

	// oh（offset_hours）で指定されている時間よりも前のイベントを取得.
	events, err := models.FindEventsBetween(ctx, time.Time{}, time.Now().Add(time.Duration(-1*offsetHours)*time.Hour))
	if err != nil {
		render.JSON(http.StatusInternalServerError, map[string]any{"error": err})
		return
	}
	if len(events) == 0 {
		render.JSON(http.StatusOK, map[string]any{"offset_hours": offsetHours, "events": events})
		return
	}

	latest := events[0]

	all := []models.Equip{}
	// unreported := []models.Equip{}
	unreported := []string{}

	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, map[string]any{"error": err})
		return
	}
	defer client.Close()

	if _, err = client.GetAll(ctx, datastore.NewQuery(models.KindEquip), &all); err != nil {
		render.JSON(http.StatusInternalServerError, map[string]any{"error": err})
		return
	}

	for _, equip := range all {
		if !equip.ForPractice && !equip.ForGame {
			continue
		}
		query := datastore.NewQuery(models.KindCustody).Ancestor(equip.Key).Order("-Timestamp").Limit(1)
		if _, err = client.GetAll(ctx, query, &equip.History); err != nil {
			render.JSON(http.StatusInternalServerError, map[string]any{"error": err})
			return
		}
		if len(equip.History) == 0 {
			unreported = append(unreported, fmt.Sprintf("・%s [報告ゼロ]", equip.Name))
		} else if equip.History[0].Timestamp < latest.Google.EndTime {
			// unreported = append(unreported, fmt.Sprintf("・%s [直近:<@%s>]", equip.Name, equip.History[0].MemberID))
			unreported = append(unreported, fmt.Sprintf("・<%s/equips/%d|%s>", server.HubBaseURL(), equip.ID, equip.Name))
		}
	}

	api := slack.New(os.Getenv("SLACK_BOT_USER_OAUTH_TOKEN"))
	channel := req.URL.Query().Get("channel")
	if _, _, err := api.PostMessage(channel, slack.MsgOptionBlocks(
		slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf(
			"以下の備品は「%s: %s」から現時点までで備品報告の無いものです。必要なら代理報告機能を使って、備品の所在を登録してください。",
			latest.Google.Start().Format("2006/01/02"),
			latest.Google.Title,
		), false, false), nil, nil),
		slack.NewDividerBlock(),
		slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType, strings.Join(
			unreported, "\n",
		), false, false), nil, nil),
		slack.NewDividerBlock(),
		slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf(
			"%s/equips/report", server.HubBaseURL(),
		), false, false), nil, nil),
	)); err != nil {
		log.Println("[ERROR]", 9001, err.Error())
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err})
		return
	}
	render.JSON(http.StatusOK, map[string]any{
		"event":      latest,
		"unreported": unreported,
	})
}
