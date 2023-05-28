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

	// 3) 全Equipsを取得する
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

	// 4) 必要なequipの場合は所有者を取得する
	targets := []models.Equip{}
	for i, eq := range equips {
		if !eq.ShouldBringFor(ev) {
			continue
		}
		equips[i].ID = eq.Key.ID
		// 最新のHistoryだけ収集する
		query := datastore.NewQuery(models.KindCustody).Ancestor(eq.Key).Order("-Timestamp").Limit(1)
		client.GetAll(ctx, query, &equips[i].History) // エラーは無視してよい
		targets = append(targets, equips[i])
	}

	// 5) 該当するEquipsの所持者に対してSlackにメンションを送る
	pats, err := ev.Participations()
	if err != nil {
		log.Println("[ERROR]", 8004, err.Error())
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
	}
	alloc := summarizeEquipAllocForTheEvent(ev, targets, pats)

	api := slack.New(os.Getenv("SLACK_BOT_USER_OAUTH_TOKEN"))
	msg := buildEquipsReminderMsg(alloc)
	if _, _, err := api.PostMessageContext(ctx, "general", msg); err != nil {
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

func EquipsRemindReportAfterEvent(w http.ResponseWriter, req *http.Request) {

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

	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, map[string]any{"error": err})
		return
	}
	defer client.Close()

	// 全件取得
	all := []models.Equip{}
	if _, err = client.GetAll(ctx, datastore.NewQuery(models.KindEquip), &all); err != nil {
		render.JSON(http.StatusInternalServerError, map[string]any{"error": err})
		return
	}

	targets := []models.Equip{}
	for _, equip := range all {
		if equip.ShouldBringFor(ev) {
			targets = append(targets, equip)
		}
	}

	blocks := []slack.Block{
		slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType, strings.Join([]string{
			fmt.Sprintf("@channel お疲れさまでした！ *%s*", ev.Google.Title),
			"備品を持って帰って頂いた方は、以下のフォームにご回答いただくようお願いいたします！",
			fmt.Sprintf("複数の備品回収を一括で報告する場合は、<%s/equips/report|こちらのリンク>から登録ください！ ", server.HubBaseURL()),
		}, "\n"), false, false), nil, nil),
	}

	api := slack.New(os.Getenv("SLACK_BOT_USER_OAUTH_TOKEN"))
	channel := req.URL.Query().Get("channel")
	if channel == "" {
		channel = "general"
	}

	_, ts, err := api.PostMessage(channel, slack.MsgOptionBlocks(blocks...))
	if err != nil {
		render.JSON(http.StatusInternalServerError, map[string]any{"error": err})
		return
	}

	for _, equip := range targets {
		text := equip.Name
		block := slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, text, false, false), nil,
			slack.NewAccessory(slack.NewOptionsSelectBlockElement("users_select", nil, fmt.Sprintf("equip_unreported/?eid=%d&ev=%s", equip.Key.ID, ev.Google.Title))),
		)
		_, _, err = api.PostMessage(channel, slack.MsgOptionBlocks(block), slack.MsgOptionTS(ts))
		if err != nil {
			render.JSON(http.StatusInternalServerError, map[string]any{"error": err})
			return
		}
	}

	render.JSON(http.StatusOK, map[string]any{
		"event":   ev.Google,
		"targets": targets,
		"channel": channel,
	})

}

func EquipsScanUnreported(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w, true)
	offsetHours, err := strconv.Atoi(req.URL.Query().Get("oh"))
	if err != nil {
		render.JSON(http.StatusBadRequest, map[string]any{"error": err})
		return
	}
	ctx := req.Context()

	// 最大10件のイベントをすべて取得
	events, err := models.FindEventsBetween(ctx, time.Time{}, time.Now())
	if err != nil {
		render.JSON(http.StatusInternalServerError, map[string]any{"error": err})
		return
	}
	if len(events) == 0 {
		render.JSON(http.StatusOK, map[string]any{"offset_hours": offsetHours, "events": events})
		return
	}

	latest := events[0]
	if time.Now().Add(-1 * time.Duration(offsetHours) * time.Hour).Before(latest.Google.Start()) {
		render.JSON(http.StatusOK, map[string]any{
			"offset_hours": offsetHours,
			"latest":       latest.Google,
			"start":        latest.Google.Start(),
			"message":      fmt.Sprintf("このイベントは、発生から%d時間経っていないので、まだスキャンしない", offsetHours),
		})
		return
	}

	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, map[string]any{"error": err})
		return
	}
	defer client.Close()

	// 全件取得
	all := []models.Equip{}
	if _, err = client.GetAll(ctx, datastore.NewQuery(models.KindEquip), &all); err != nil {
		render.JSON(http.StatusInternalServerError, map[string]any{"error": err})
		return
	}

	// 未報告をスキャン
	unreported := []models.Equip{}
	for _, equip := range all {
		if !equip.ShouldBringFor(latest) {
			continue
		}
		query := datastore.NewQuery(models.KindCustody).Ancestor(equip.Key).Order("-Timestamp").Limit(1)
		if _, err = client.GetAll(ctx, query, &equip.History); err != nil {
			render.JSON(http.StatusInternalServerError, map[string]any{"error": err})
			return
		}
		if equip.HasBeenUpdatedSince(latest.Google.Start()) {
			continue
		}
		unreported = append(unreported, equip)
	}

	if len(unreported) == 0 {
		render.JSON(http.StatusOK, map[string]any{
			"events":       len(events),
			"offset_hours": offsetHours,
			"latest_event": latest.Google,
			"unreported":   unreported,
		})
		return
	}

	blocks := []slack.Block{
		slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf(
			"以下の備品は<%s/events/%s|「%s: %s」>から現時点までで備品報告の無いものです。現在の備品の所在を登録してください。",
			server.HubBaseURL(), latest.Google.ID,
			latest.Google.Start().Format("2006/01/02"),
			latest.Google.Title,
		), false, false), nil, nil),
	}

	api := slack.New(os.Getenv("SLACK_BOT_USER_OAUTH_TOKEN"))
	channel := req.URL.Query().Get("channel")
	_, ts, err := api.PostMessage(channel, slack.MsgOptionBlocks(blocks...))
	if err != nil {
		render.JSON(http.StatusInternalServerError, map[string]any{"error": err})
		return
	}

	for _, equip := range unreported {
		text := equip.Name
		if len(equip.History) > 0 {
			if m, err := models.GetMemberInfoByCache(ctx, equip.History[0].MemberID); err == nil {
				text += fmt.Sprintf("\n(前回: <%s/equips/%d|%s>)", server.HubBaseURL(), equip.Key.ID, m.Name())
			}
		}
		block := slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, text, false, false), nil,
			slack.NewAccessory(slack.NewOptionsSelectBlockElement("users_select", nil, fmt.Sprintf("equip_unreported/?eid=%d&ev=%s", equip.Key.ID, latest.Google.Title))),
		)
		_, _, err = api.PostMessage(channel, slack.MsgOptionBlocks(block), slack.MsgOptionTS(ts))
		if err != nil {
			render.JSON(http.StatusInternalServerError, map[string]any{"error": err})
			return
		}
	}

	render.JSON(http.StatusOK, map[string]any{
		"events":       len(events),
		"offset_hours": offsetHours,
		"latest_event": latest.Google,
		"unreported":   unreported,
	})

}
