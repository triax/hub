package tasks

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/otiai10/marmoset"
	"github.com/slack-go/slack"
	"github.com/triax/hub/server"
	"github.com/triax/hub/server/models"
)

func ConditionFrom(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	render := marmoset.Render(w, true)

	channel := req.URL.Query().Get("channel")
	if channel == "" {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": "`channel` is required"})
		return
	}
	position := req.URL.Query().Get("position")
	if position == "" {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": "`position` is required"})
		return
	}
	label := req.URL.Query().Get("label")
	if label == "" {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": "`label` is required"})
		return
	}

	f, t, err := defineTimeRangeByRequest(time.Now(), req)
	if err != nil {
		log.Println("[ERROR]", 9001, err.Error())
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}
	events, err := models.FindEventsBetween(ctx, f, t)
	if err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"events": events, "error": err})
		return
	}
	if len(events) == 0 { // 該当イベント無し
		render.JSON(http.StatusOK, marmoset.P{"events": events, "message": "not found"})
		return
	}
	ev := events[0]
	if ev.ShouldSkipReminders() {
		render.JSON(http.StatusOK, marmoset.P{"events": events, "message": "not found"})
		return
	}
	title := ev.Google.Title

	blocks := createConditioningMessageBlocks(ev, label, position, "", "")
	api := slack.New(os.Getenv("SLACK_BOT_USER_OAUTH_TOKEN"))
	if ch, ts, err := api.PostMessageContext(ctx, channel, slack.MsgOptionBlocks(blocks...)); err != nil {
		log.Println("[ERROR]", 8005, err.Error())
		render.JSON(http.StatusInternalServerError, marmoset.P{"success": false, "error": err.Error(), "blocks": blocks})
	} else {
		render.JSON(http.StatusOK, marmoset.P{"success": true, "blocks": blocks, "channel": channel, "title": title})
		blocks := createConditioningMessageBlocks(ev, label, position, ch, ts)
		api.UpdateMessage(ch, ts, slack.MsgOptionBlocks(blocks...))
	}
}

func createConditioningMessageBlocks(ev models.Event, label, position, ch, timestamp string) (blocks []slack.Block) {
	text := fmt.Sprintf("コンディショニングチェックシートのご入力宜しくお願い致します！ *【%s】*\n%s", strings.ToUpper(position), ev.Google.Title)
	link := fmt.Sprintf("%s/redirect/conditioning-form", server.HubBaseURL())
	query := url.Values{"label": []string{label}, "position": []string{position}}
	if timestamp != "" && ch != "" {
		query.Add("slack_timestamp", timestamp)
		query.Add("response_channel", ch)
	}
	switch label {
	case "before":
		text = fmt.Sprintf("*【運動前】* %s", text)
	case "after":
		text = fmt.Sprintf("*【運動後】* %s", text)
	}
	blocks = append(
		blocks,
		slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType, text, false, false), nil, nil),
		slack.NewActionBlock(
			"",
			&slack.ButtonBlockElement{
				Type:  slack.METButton,
				Text:  slack.NewTextBlockObject(slack.PlainTextType, "チェックシートを開く", false, false),
				Style: slack.StylePrimary,
				URL:   link + "?" + query.Encode(),
			},
		),
	)
	return
}
