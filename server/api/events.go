package api

import (
	"bytes"
	"encoding/json"
	"html/template"
	"net/http"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/go-chi/chi/v5"
	"github.com/otiai10/marmoset"
	"github.com/slack-go/slack"
	"github.com/triax/hub/server/filters"
	"github.com/triax/hub/server/models"
)

var (
	TplLastMinuteRSVPChange = template.Must(template.New("").Parse(`直近のイベントに欠席回答への変更がありました.
{{.member.Slack.Profile.RealName}} ({{if .member.Slack.Profile.Title}}{{.member.Slack.Profile.Title}}{{else}}ポジション未設定{{end}})
[{{.event.Google.Title}}]`))
)

func GetEvent(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	id := chi.URLParam(req, "id")
	event := models.Event{}
	key := datastore.NameKey(models.KindEvent, id, nil)
	if err := client.Get(ctx, key, &event); err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}
	render.JSON(http.StatusOK, event)
}

func DeleteEvent(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()
	id := chi.URLParam(req, "id")
	key := datastore.NameKey(models.KindEvent, id, nil)
	if err := client.Delete(ctx, key); err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}
	render.JSON(http.StatusOK, marmoset.P{"id": id, "ok": true})
}

func ListEvents(w http.ResponseWriter, req *http.Request) {

	render := marmoset.Render(w)
	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	events := []models.Event{}
	query := datastore.NewQuery(models.KindEvent).
		Filter("Google.StartTime >", time.Now().Unix()*1000).
		Order("Google.StartTime")

	if _, err := client.GetAll(ctx, query, &events); err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	render.JSON(http.StatusOK, events)
}

func AnswerEvent(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	slackID := filters.GetSessionUserContext(req)

	body := struct {
		Event struct {
			ID string `json:"id"`
		} `json:"event"`
		Type   models.ParticipationType `json:"type"`
		Params map[string]interface{}   `json:"params"`
	}{}
	defer req.Body.Close()
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}

	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	member := models.Member{}
	if err := client.Get(ctx,
		datastore.NameKey(models.KindMember, slackID, nil),
		&member,
	); err != nil && !models.IsFiledMismatch(err) {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}

	event := models.Event{}
	key := datastore.NameKey(models.KindEvent, body.Event.ID, nil)
	if err := client.Get(ctx, key, &event); err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}
	if event.ParticipationsJSONString == "" {
		event.ParticipationsJSONString = "{}"
	}
	parts := models.Participations{}
	if err := json.NewDecoder(strings.NewReader(event.ParticipationsJSONString)).Decode(&parts); err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}

	// 直前の欠席連絡か?
	if p, ok := parts[slackID]; ok && p.Type.JoinAnyhow() && body.Type == models.PTAbsent {
		if time.Until(event.Google.Start()) < 48*time.Hour { // 直前とは「48時間前を超えたら」
			token := os.Getenv("SLACK_BOT_USER_OAUTH_TOKEN")
			channel, msg := buildSlackMessageOfLastMinuteRSVPChange(member, event)
			slack.New(token).PostMessageContext(ctx, channel, msg...) // は〜エラー見るのめんどくせ
		}
	}

	parts[slackID] = models.Participation{
		Type:    body.Type,
		Params:  body.Params,
		Name:    member.Slack.Profile.RealName,
		Picture: member.Slack.Profile.Image512,
		Title:   member.Slack.Profile.Title,
	}
	b, err := json.Marshal(parts)
	if err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}
	event.ParticipationsJSONString = string(b)
	if _, err := client.Put(ctx, key, &event); err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}

	render.JSON(http.StatusAccepted, event)
}

func buildSlackMessageOfLastMinuteRSVPChange(m models.Member, e models.Event) (string, []slack.MsgOption) {
	buf := bytes.NewBuffer(nil)
	if err := TplLastMinuteRSVPChange.Execute(buf, map[string]interface{}{"member": m, "event": e}); err != nil {
		return "tech", []slack.MsgOption{
			slack.MsgOptionBlocks(slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType, err.Error(), false, false), nil, nil)),
		}
	}
	return "practice", []slack.MsgOption{
		slack.MsgOptionBlocks(slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType, buf.String(), false, false), nil, nil)),
	}
}
