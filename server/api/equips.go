package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/go-chi/chi/v5"
	"github.com/otiai10/marmoset"
	"github.com/triax/hub/server/models"
)

func ListEquips(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	equips := []models.Equip{}
	query := datastore.NewQuery(models.KindEquip)
	if _, err := client.GetAll(ctx, query, &equips); err != nil && !models.IsFiledMismatch(err) {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	for i, e := range equips {
		equips[i].ID = e.Key.ID

		// 最新のHistoryだけ収集する
		query := datastore.NewQuery(models.KindCustody).Ancestor(e.Key).Order("-Timestamp").Limit(1)
		client.GetAll(ctx, query, &equips[i].History) // エラーは無視してよい
	}

	if req.URL.Query().Get("cached") == "1" {
		age := 4 * 60 * 60 // 4時間
		w.Header().Add("Cache-Control", fmt.Sprintf("public, max-age=%d, immutable", age))
	}

	render.JSON(http.StatusOK, equips)
}

func GetEquip(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	equip := models.Equip{}
	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}
	key := datastore.IDKey(models.KindEquip, id, nil)
	if err := client.Get(ctx, key, &equip); err != nil && !models.IsFiledMismatch(err) {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	equip.ID = key.ID

	query := datastore.NewQuery(models.KindCustody).Ancestor(key).Order("-Timestamp")

	if _, err := client.GetAll(ctx, query, &equip.History); err != nil && !models.IsFiledMismatch(err) {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	if req.URL.Query().Get("cached") == "1" {
		age := 4 * 60 * 60 // 4時間
		w.Header().Add("Cache-Control", fmt.Sprintf("public, max-age=%d, immutable", age))
	}

	render.JSON(http.StatusOK, equip)
}

func CreateEquipItem(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	defer req.Body.Close()
	equip := models.Equip{}
	if err := json.NewDecoder(req.Body).Decode(&equip); err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}
	key := datastore.IncompleteKey(models.KindEquip, nil)
	equip.Key = key

	if created, err := client.Put(ctx, key, &equip); err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	} else {
		equip.Key = created
		equip.ID = created.ID
	}

	render.JSON(http.StatusCreated, equip)
}

func DeleteEquip(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}
	equip := models.Equip{}
	key := datastore.IDKey(models.KindEquip, id, nil)
	if err := client.Get(ctx, key, &equip); err != nil && !models.IsFiledMismatch(err) {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	equip.Key = key
	equip.ID = key.ID

	query := datastore.NewQuery(models.KindCustody).Ancestor(key)
	if keys, err := client.GetAll(ctx, query, &equip.History); err != nil && !models.IsFiledMismatch(err) {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	} else {
		if err := client.DeleteMulti(ctx, keys); err != nil {
			render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
			return
		}
	}

	if err := client.Delete(ctx, equip.Key); err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	render.JSON(http.StatusAccepted, equip)
}

func EquipCustodyReport(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	body := struct {
		IDs      []int64 `json:"ids"`
		MemberID string  `json:"member_id"`
		Comment  string  `json:"comment"`
	}{}
	defer req.Body.Close()
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}

	keys := []*datastore.Key{}
	custodies := []*models.Custody{}
	for _, id := range body.IDs {
		keys = append(keys, datastore.IncompleteKey(
			models.KindCustody,
			datastore.IDKey(models.KindEquip, id, nil),
		))
		custodies = append(custodies, &models.Custody{
			MemberID:  body.MemberID,
			Timestamp: time.Now().Unix() * 1000,
		})
	}

	if inserted, err := client.PutMulti(ctx, keys, custodies); err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	} else {
		for i, key := range inserted {
			custodies[i].Key = key
		}
	}

	render.JSON(http.StatusAccepted, custodies)
}
