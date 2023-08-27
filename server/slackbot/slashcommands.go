package slackbot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"cloud.google.com/go/datastore"
	"github.com/triax/hub/server/models"
)

var (
	mentionExp = regexp.MustCompile(`@(?P<name>[\S]+)`)
)

func (bot Bot) SlashCommands(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	defer req.Body.Close()
	text := req.Form.Get("text")
	names := []string{}
	idx := mentionExp.SubexpIndex("name")
	for _, m := range mentionExp.FindAllStringSubmatch(text, -1) {
		fmt.Println(m[idx])
		names = append(names, m[idx])
	}
	if len(names) == 0 {
		http.Post(req.Form.Get("response_url"), "application/json", strings.NewReader(`{
			"text": "誰に対するありがとうか、メンションで指定してください。本人には匿名のDMで通知されます。"
		}`))
		return
	}

	ctx := context.Background()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		b, err := json.Marshal(map[string]string{
			"text": fmt.Sprintf("データストアに接続できませんでした。 @ten までご連絡ください。\n```%s```", err.Error()),
		})
		if err != nil {
			fmt.Println("[ERROR]", 6003, err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		http.Post(req.Form.Get("response_url"), "application/json", bytes.NewReader(b))
		return
	}
	defer client.Close()

	users := []models.Member{}
	query := datastore.NewQuery(models.KindMember).FilterField("Slack.Name", "in", strings.Join(names, ","))
	if _, err := client.GetAll(ctx, query, &users); err != nil {
		b, err := json.Marshal(map[string]string{
			"text": fmt.Sprintf("データストアからのデータ取得に失敗しました。 @ten までご連絡ください。\n```%s```", err.Error()),
		})
		if err != nil {
			fmt.Println("[ERROR]", 6003, err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		http.Post(req.Form.Get("response_url"), "application/json", bytes.NewReader(b))
		return
	}

	for _, u := range users {
		fmt.Printf("%+v\n", u)
	}

	fmt.Println(names)
	w.WriteHeader(http.StatusOK)
}
