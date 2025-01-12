package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"cloud.google.com/go/datastore"
	"github.com/go-chi/chi/v5"
	"github.com/otiai10/marmoset"
	"github.com/triax/hub/server/models"
)

func GetAllNumbers(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	q := datastore.NewQuery(models.KindNumber)
	numbers := []models.PlayerNumber{}
	if _, err := client.GetAll(ctx, q, &numbers); err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	if len(numbers) != 100 {
		// 100件ない場合は、100件埋める.
		// ただし、背番号0から99までの連番であることが前提.
		for i := 0; i < 100; i++ {
			found := false
			for _, n := range numbers {
				if n.Number == i {
					found = true
					break
				}
			}
			if !found {
				numbers = append(numbers, models.PlayerNumber{Number: i})
			}
		}
	}

	// age := 2 * 60 * 60 // 2時間
	// w.Header().Add("Cache-Control", fmt.Sprintf("public, max-age=%d, immutable", age))
	render.JSON(http.StatusOK, numbers)
}

func AssignPlayerNumber(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	playernumber := models.PlayerNumber{}

	num := chi.URLParam(req, "num")
	if num == "" {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": "number is required"})
		return
	}
	if n, err := strconv.ParseInt(num, 10, 8); err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": fmt.Sprintf("number must be int: %s", num)})
		return
	} else {
		playernumber.Number = int(n)
	}

	// あってもなくてもいいので、エラーはとりあえず無視
	client.Get(ctx, datastore.NameKey(models.KindNumber, num, nil), &playernumber)

	body := models.PlayerNumber{}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}

	player := models.Member{}
	key := datastore.NameKey(models.KindMember, body.PlayerID, nil)
	if err := client.Get(ctx, key, &player); err != nil && !models.IsFiledMismatch(err) {
		render.JSON(http.StatusNotFound, marmoset.P{"error": fmt.Sprintf("player not found: %s", body.PlayerID)})
		return
	}

	prevs := []models.Member{}
	query := datastore.NewQuery(models.KindMember).FilterField("Number", "=", playernumber.Number)
	if _, err := client.GetAll(ctx, query, &prevs); err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	// Deprive previous players of the number...!
	if len(prevs) > 0 {
		if _, err := client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
			for _, prev := range prevs {
				if prev.Slack.ID == body.PlayerID {
					continue
				}
				prev.Number = nil
				if _, err := tx.Put(datastore.NameKey(models.KindMember, prev.Slack.ID, nil), &prev); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
			return
		}
	}

	// Assing the number to the player
	player.Number = &playernumber.Number
	playernumber.PlayerID = player.Slack.ID
	if _, err := client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		if _, err := tx.Put(datastore.NameKey(models.KindMember, player.Slack.ID, nil), &player); err != nil {
			return err
		}
		if _, err := tx.Put(datastore.NameKey(models.KindNumber, num, nil), &playernumber); err != nil {
			return err
		}
		return nil
	}); err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
}

func DeprivePlayerNumber(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	num := chi.URLParam(req, "num")
	if num == "" {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": "number is required"})
		return
	}

	playernumber := models.PlayerNumber{}
	if err := client.Get(ctx, datastore.NameKey(models.KindNumber, num, nil), &playernumber); err != nil {
		render.JSON(http.StatusNotFound, marmoset.P{"error": fmt.Sprintf("number not found: %s", num)})
		return
	}

	player := models.Member{}
	if err := client.Get(ctx, datastore.NameKey(models.KindMember, playernumber.PlayerID, nil), &player); err != nil {
		render.JSON(http.StatusNotFound, marmoset.P{"error": fmt.Sprintf("player not found: %s", playernumber.PlayerID)})
		return
	}

	// Deprive the number from the player
	player.Number = nil
	playernumber.PlayerID = ""
	if _, err := client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		if _, err := tx.Put(datastore.NameKey(models.KindMember, player.Slack.ID, nil), &player); err != nil {
			return err
		}
		if _, err := tx.Put(datastore.NameKey(models.KindNumber, num, nil), &playernumber); err != nil {
			return err
		}
		return nil
	}); err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
}
