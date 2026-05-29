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
	"github.com/triax/hub/server/models"
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

	if len(events) == 0 {
		render.JSON(http.StatusOK, marmoset.P{"events": events, "message": "not found"})
		return
	}
	ev := events[0]

	if ev.ShouldSkipReminders(models.RTEquipment) {
		render.JSON(http.StatusOK, marmoset.P{"events": events, "message": "not found"})
		return
	}

	// 2) 全Equipsを取得する
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

	// 3) 持ち帰り管理かつ対象イベント向け備品のホルダーをグルーピング
	byHolder := map[string][]models.Equip{}
	for i, eq := range equips {
		if eq.StorageType == models.StorageTypeWarehouse {
			continue
		}
		if !eq.ShouldBringFor(ev) {
			continue
		}
		equips[i].ID = eq.Key.ID
		query := datastore.NewQuery(models.KindCustody).Ancestor(eq.Key).Order("-Timestamp").Limit(1)
		client.GetAll(ctx, query, &equips[i].History) // エラーは無視してよい
		if len(equips[i].History) == 0 {
			log.Printf("[WARN] 誰も管理していない: %s", eq.Name)
			continue
		}
		uid := equips[i].History[0].MemberID
		byHolder[uid] = append(byHolder[uid], equips[i])
	}

	if len(byHolder) == 0 {
		render.JSON(http.StatusOK, marmoset.P{"message": "no targets"})
		return
	}

	// 4) ホルダーごとに個別DM送信
	api := slack.New(os.Getenv("SLACK_BOT_USER_OAUTH_TOKEN"))
	dmed := []string{}
	for uid, eqs := range byHolder {
		ch, _, _, err := api.OpenConversation(&slack.OpenConversationParameters{
			Users: []string{uid},
		})
		if err != nil {
			log.Printf("[ERROR] 8004 OpenConversation %s: %v", uid, err)
			continue
		}
		names := make([]string, 0, len(eqs))
		for _, eq := range eqs {
			if eq.NeedsCharge() {
				names = append(names, ":electric_plug::zap: _"+eq.Name+"_")
			} else {
				names = append(names, "・"+eq.Name)
			}
		}
		msg := fmt.Sprintf(
			"明日の *%s* にて以下の備品をお持ちください :bow:\n%s\n※ご欠席の場合は参加者への引き渡しをお願いします。",
			ev.Google.Title,
			strings.Join(names, "\n"),
		)
		if _, _, err := api.PostMessage(ch.ID, slack.MsgOptionText(msg, false)); err != nil {
			log.Printf("[ERROR] 8005 PostMessage DM to %s: %v", uid, err)
			continue
		}
		dmed = append(dmed, uid)
	}

	render.JSON(http.StatusOK, marmoset.P{"event": ev.Google.Title, "dmed": dmed})
}

func EquipsRemindReportAfterEvent(w http.ResponseWriter, req *http.Request) {

	ctx := req.Context()
	render := marmoset.Render(w, true)

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

	if len(events) == 0 {
		render.JSON(http.StatusOK, marmoset.P{"events": events, "message": "not found"})
		return
	}

	ev := events[0]

	if ev.ShouldSkipReminders(models.RTEquipment) {
		render.JSON(http.StatusOK, marmoset.P{"events": events, "message": "not found"})
		return
	}

	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, map[string]any{"error": err})
		return
	}
	defer client.Close()

	all := []models.Equip{}
	if _, err = client.GetAll(ctx, datastore.NewQuery(models.KindEquip), &all); err != nil {
		render.JSON(http.StatusInternalServerError, map[string]any{"error": err})
		return
	}

	targets := []models.Equip{}
	for _, equip := range all {
		if equip.StorageType == models.StorageTypeWarehouse {
			continue
		}
		if equip.ShouldBringFor(ev) {
			targets = append(targets, equip)
		}
	}

	if len(targets) == 0 {
		render.JSON(http.StatusOK, map[string]any{"event": ev.Google, "message": "no targets"})
		return
	}

	// 全備品を1投稿にまとめる（スレッド不使用）
	headerText := fmt.Sprintf(
		"<!channel> お疲れさまでした！ *%s*\n備品を持ち帰った方は、以下から保管者を登録してください :bow:",
		ev.Google.Title,
	)
	blocks := []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, headerText, false, false),
			nil, nil,
		),
	}
	for _, equip := range targets {
		blocks = append(blocks, slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, equip.Name, false, false),
			nil,
			slack.NewAccessory(slack.NewOptionsSelectBlockElement(
				"users_select", nil,
				fmt.Sprintf("equip_unreported/?eid=%d&ev=%s", equip.Key.ID, ev.Google.Title),
			)),
		))
	}

	api := slack.New(os.Getenv("SLACK_BOT_USER_OAUTH_TOKEN"))
	channel := req.URL.Query().Get("channel")
	if channel == "" {
		channel = "general"
	}

	// text フィールドにマーカーを設定（EquipsScanUnreported が ts 検索に使用）
	_, _, err = api.PostMessage(
		channel,
		slack.MsgOptionText("equip-report-reminder", false),
		slack.MsgOptionBlocks(blocks...),
	)
	if err != nil {
		render.JSON(http.StatusInternalServerError, map[string]any{"error": err})
		return
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

	all := []models.Equip{}
	if _, err = client.GetAll(ctx, datastore.NewQuery(models.KindEquip), &all); err != nil {
		render.JSON(http.StatusInternalServerError, map[string]any{"error": err})
		return
	}

	// 未報告をスキャン（倉庫管理はスキップ）
	unreported := []models.Equip{}
	for _, equip := range all {
		if equip.StorageType == models.StorageTypeWarehouse {
			continue
		}
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

	api := slack.New(os.Getenv("SLACK_BOT_USER_OAUTH_TOKEN"))
	channel := req.URL.Query().Get("channel")
	if channel == "" {
		channel = "general"
	}

	// 直近の EquipsRemindReportAfterEvent 投稿の ts を検索
	reportTS, err := findRecentReportPostTS(api, channel)
	if err != nil {
		log.Printf("[WARN] 9003 history search failed: %v", err)
	}
	if reportTS == "" {
		// 直近の報告投稿が見つからない場合はスキップ
		render.JSON(http.StatusOK, map[string]any{
			"message": "no recent report post found, skipping",
		})
		return
	}

	// 未報告備品を1メッセージにまとめてスレッド返信
	blocks := []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType,
				"以下の備品の保管者がまだ未登録です :mag: 心当たりのある方は登録をお願いします。",
				false, false,
			),
			nil, nil,
		),
	}
	for _, equip := range unreported {
		mention := ""
		if len(equip.History) > 0 {
			mention = fmt.Sprintf("<@%s> ", equip.History[0].MemberID)
		}
		blocks = append(blocks, slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType,
				fmt.Sprintf("%s%s", mention, equip.Name),
				false, false,
			),
			nil,
			slack.NewAccessory(slack.NewOptionsSelectBlockElement(
				"users_select", nil,
				fmt.Sprintf("equip_unreported/?eid=%d&ev=%s", equip.Key.ID, latest.Google.Title),
			)),
		))
	}

	if _, _, err = api.PostMessage(
		channel,
		slack.MsgOptionTS(reportTS),
		slack.MsgOptionBlocks(blocks...),
	); err != nil {
		render.JSON(http.StatusInternalServerError, map[string]any{"error": err})
		return
	}

	render.JSON(http.StatusOK, map[string]any{
		"events":       len(events),
		"offset_hours": offsetHours,
		"latest_event": latest.Google,
		"unreported":   unreported,
		"thread_ts":    reportTS,
	})
}

// findRecentReportPostTS は channel の直近100件のメッセージから
// EquipsRemindReportAfterEvent が投稿したルートメッセージの ts を返す。
// 見つからない場合は空文字を返す。
func findRecentReportPostTS(api *slack.Client, channel string) (string, error) {
	hist, err := api.GetConversationHistory(&slack.GetConversationHistoryParameters{
		ChannelID: channel,
		Limit:     100,
	})
	if err != nil {
		return "", fmt.Errorf("GetConversationHistory: %w", err)
	}
	for _, msg := range hist.Messages {
		if msg.Text == "equip-report-reminder" {
			return msg.Timestamp, nil
		}
	}
	return "", nil
}
