package tasks

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/otiai10/marmoset"
	"github.com/slack-go/slack"
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

func EquipsRemindPractice(w http.ResponseWriter, req *http.Request) {

	ctx := req.Context()
	render := marmoset.Render(w, true)

	// 1) 直近1週間以内の直近のEventを1件だけ取得する
	// 1-a) Google Datasotreへのアクセスをするクライアントを作成する
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		log.Println("[ERROR]", 8001, err.Error())
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	// 1-b) Eventの取得
	events := []models.Event{}
	query := datastore.NewQuery(models.KindEvent).
		Filter("Google.StartTime >", time.Now().Unix()*1000).
		Filter("Google.StartTime <=", time.Now().Add(7*24*time.Hour).Unix()*1000).
		Order("Google.StartTime").
		Limit(1)
	if _, err := client.GetAll(ctx, query, &events); err != nil {
		log.Println("[ERROR]", 8002, err.Error())
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	// 2) 1のEventが無ければ終了
	if len(events) == 0 {
		render.JSON(http.StatusNotFound, marmoset.P{"events": events})
		return
	}

	// 3) 全Equipsの所持者を取得する
	equips := []models.Equip{}
	if _, err := client.GetAll(ctx,
		datastore.NewQuery(models.KindEquip),
		&equips,
	); err != nil && !models.IsFiledMismatch(err) {
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
	event := events[0]
	pats, err := event.Participations()
	if err != nil {
		log.Println("[ERROR]", 8004, err.Error())
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
	}
	alloc := summarizeEquipAllocForTheEvent(event, equips, pats)

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
					"備品を持って帰ってくれている皆さまへ\n%s にて以下の備品を持ってきていただけるようお願いします :bow:",
					alloc.Event.Google.Title,
				),
				false, false,
			), nil, nil,
		),
	}
	for uid, equips := range alloc.OK {
		names := []string{}
		for _, e := range equips {
			names = append(names, "*"+e.Name+"*")
		}
		blocks = append(blocks,
			slack.NewContextBlock("",
				slack.NewTextBlockObject(
					slack.MarkdownType,
					fmt.Sprintf("<@%s>", uid)+"\n"+strings.Join(names, " || "),
					false, false,
				),
			),
		)
	}
	return slack.MsgOptionBlocks(blocks...)
}
