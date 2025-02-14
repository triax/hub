package tasks

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/otiai10/marmoset"
	"github.com/slack-go/slack"
	"github.com/triax/hub/server/models"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

const (
	eventFetchDurationMonths = 6 // 半年までのイベントを取得する
)

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

	now := time.Now()
	all, err := service.Events.List(id).
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(now.Format(time.RFC3339)).
		TimeMax(now.AddDate(0, eventFetchDurationMonths, 0).Format(time.RFC3339)).
		OrderBy("startTime").
		Do()
	if err != nil {
		fmt.Println("[ERROR]", 7002, err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err.Error())
		return
	}

	// #ignore と、#mtg が含まれるイベントは、そもそもHubに入れない
	targets := []calendar.Event{}
	ignored := []calendar.Event{}
	for _, item := range all.Items {
		switch {
		case models.EventExpressionIgnore.MatchString(item.Summary):
			ignored = append(ignored, *item)
		case models.EventExpressionMeeting.MatchString(item.Summary):
			ignored = append(ignored, *item)
		default:
			targets = append(targets, *item)
		}
	}

	if req.URL.Query().Get("dry") != "" {
		marmoset.RenderJSON(w, 200, marmoset.P{
			"targets": targets,
			"ignored": ignored,
		})
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

	var created, updated int
	if _, err := client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		for _, item := range targets {
			ev := models.Event{}
			key := datastore.NameKey(models.KindEvent, item.Id, nil)
			if err := tx.Get(key, &ev); err != nil {
				fmt.Printf("[DEBUG] NEW EVENT: %+v\n", item)
				created += 1
			} else {
				updated += 1
			}
			ev.Google = models.CreateEventFromCalendarAPI(&item)
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
		"events": map[string]any{
			"total":      len(all.Items),
			"ignored":    len(ignored),
			"created":    created,
			"updated":    updated,
			"validation": len(all.Items) == len(ignored)+created+updated,
		},
	})
}

func CronFetchSlackMembers(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	token := os.Getenv("SLACK_BOT_USER_OAUTH_TOKEN")
	api := slack.New(token)

	team, err := api.GetTeamInfoContext(ctx)
	if err != nil {
		fmt.Println("[ERROR]", 6001, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	users, err := api.GetUsersContext(ctx)
	if err != nil {
		fmt.Println("[ERROR]", 6002, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if req.URL.Query().Get("dry") != "" {
		marmoset.RenderJSON(w, http.StatusOK, users)
		return
	}

	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		fmt.Println("[ERROR]", 6003, err.Error())
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
			member.Slack = models.ConvertSlackAPIUserToInternalUser(u)
			member.Team = models.ConvertSlackAPITeamToInternalTeam(*team)

			if _, err := tx.Put(key, &member); err != nil {
				return err
			}
			count++
			return nil
		}); err != nil {
			fmt.Println("[ERROR]", 6004, err)
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
