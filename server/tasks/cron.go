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

	t := time.Now().Format(time.RFC3339)
	events, err := service.Events.List(id).
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(t).
		MaxResults(20).
		OrderBy("startTime").
		Do()
	if err != nil {
		fmt.Println("[ERROR]", 7002, err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err.Error())
		return
	}

	if req.URL.Query().Get("dry") != "" {
		marmoset.RenderJSON(w, 200, events)
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

	if _, err := client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		for _, item := range events.Items {
			ev := models.Event{}
			key := datastore.NameKey(models.KindEvent, item.Id, nil)
			if err := tx.Get(key, &ev); err != nil {
				fmt.Printf("[DEBUG] NEW EVENT: %+v\n", item)
			}
			ev.Google = models.CreateEventFromCalendarAPI(item)
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
		"count":   len(events.Items),
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
