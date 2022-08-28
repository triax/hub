package tasks

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/otiai10/marmoset"
	"github.com/slack-go/slack"
	"github.com/triax/hub/server/models"
)

var (
	conditioningLeader       = "U029FBE284T" // 笹子さん
	conditionPrecheckHeader  = "おはようございます！\n*朝のコンディショニングチェックシートのご入力宜しくお願い致します！*\n:triax: "
	conditionPostcheckHeader = "お疲れさまでした！\n*運動後のコンディショニングチェックシートのご入力宜しくお願い致します！*\n:triax: "
	conditionCheckSheetURL   = "https://docs.google.com/forms/d/e/1FAIpQLSfQWL3aOUsZx868vyOZ88uVLSI5W10S1Q_qF7w5v6eZMCQ40g/viewform"
	conditionCheckFooter     = fmt.Sprintf("回答状況は各ポジションリーダーによってとりまとめ、 <@%s>さんへ報告されます :face_with_rolling_eyes::pray:", conditioningLeader)
)

// 練習や試合前に事前にコンディションチェックのフォーム入力を促すメッセージ
func ConditionPrecheck(w http.ResponseWriter, req *http.Request) {

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
		render.JSON(http.StatusNotFound, marmoset.P{"events": events})
		return
	}
	ev := events[0]

	if ev.ShouldSkipReminders() {
		render.JSON(http.StatusNotFound, marmoset.P{"events": events})
		return
	}

	title := ev.Google.Title

	api := slack.New(os.Getenv("SLACK_BOT_USER_OAUTH_TOKEN"))

	blocks := []slack.Block{
		slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType, conditionPrecheckHeader+title, false, false), nil, nil),
		slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType, conditionCheckSheetURL, false, false), nil, nil),
		slack.NewContextBlock("", slack.NewTextBlockObject(slack.MarkdownType, conditionCheckFooter, false, false)),
	}
	msg := slack.MsgOptionBlocks(blocks...)

	channel := req.URL.Query().Get("channel")
	if channel == "" {
		channel = "general"
	}

	if _, _, err := api.PostMessageContext(ctx, channel, msg); err != nil {
		log.Println("[ERROR]", 8005, err.Error())
		render.JSON(http.StatusInternalServerError, marmoset.P{"success": false, "error": err.Error(), "blocks": blocks})
	} else {
		render.JSON(http.StatusOK, marmoset.P{"success": true, "blocks": blocks, "channel": channel, "title": title})
	}
}

func ConditionPostcheck(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	render := marmoset.Render(w, true)

	ft, err := time.Parse("15:04", req.URL.Query().Get("from"))
	if err != nil {
		log.Println("[ERROR]", 9001, err.Error())
		render.JSON(http.StatusBadRequest, err.Error())
		return
	}
	tt, err := time.Parse("15:04", req.URL.Query().Get("to"))
	if err != nil {
		log.Println("[ERROR]", 9002, err.Error())
		render.JSON(http.StatusBadRequest, err.Error())
		return
	}

	// 本日の、指定時間に開始されているイベントを取得
	n := time.Now()
	from := time.Date(n.Year(), n.Month(), n.Day(), ft.Hour(), ft.Minute(), 0, 0, tokyo)
	to := time.Date(n.Year(), n.Month(), n.Day(), tt.Hour(), tt.Minute(), 0, 0, tokyo)

	query := datastore.NewQuery(models.KindEvent).
		Filter("Google.StartTime >=", from.Unix()*1000).
		Filter("Google.StartTime <", to.Unix()*1000).
		Order("Google.StartTime").Limit(1)

	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		log.Println("[ERROR]", 9003, err.Error())
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	events := []models.Event{}
	if _, err := client.GetAll(ctx, query, &events); err != nil {
		log.Println("[ERROR]", 9004, err.Error())
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	// 該当イベント無し
	if len(events) == 0 {
		render.JSON(http.StatusNotFound, marmoset.P{"events": events})
		return
	}

	ev := events[0]

	if ev.ShouldSkipReminders() {
		render.JSON(http.StatusNotFound, marmoset.P{"events": events})
		return
	}

	title := ev.Google.Title

	api := slack.New(os.Getenv("SLACK_BOT_USER_OAUTH_TOKEN"))

	blocks := []slack.Block{
		slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType, conditionPostcheckHeader+title, false, false), nil, nil),
		slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType, conditionCheckSheetURL, false, false), nil, nil),
		slack.NewContextBlock("", slack.NewTextBlockObject(slack.MarkdownType, conditionCheckFooter, false, false)),
	}
	msg := slack.MsgOptionBlocks(blocks...)

	channel := req.URL.Query().Get("channel")
	if channel == "" {
		channel = "general"
	}

	if _, _, err := api.PostMessageContext(ctx, channel, msg); err != nil {
		log.Println("[ERROR]", 8005, err.Error())
		render.JSON(http.StatusInternalServerError, marmoset.P{"success": false, "error": err.Error(), "blocks": blocks})
	} else {
		render.JSON(http.StatusOK, marmoset.P{"success": true, "blocks": blocks, "channel": channel, "title": title})
	}
}
