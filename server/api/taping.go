package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/go-chi/chi/v5"
	"github.com/otiai10/marmoset"
	"github.com/triax/hub/server/filters"
	"github.com/triax/hub/server/models"
)

// isTapingManager は is_admin または Slack title が /trainer/i にマッチするか判定する。
func isTapingManager(ctx context.Context, slackID string, client *datastore.Client) (bool, error) {
	member := models.Member{}
	key := datastore.NameKey(models.KindMember, slackID, nil)
	if err := client.Get(ctx, key, &member); err != nil && !models.IsFiledMismatch(err) {
		return false, err
	}
	if member.Slack.IsAdmin {
		return true, nil
	}
	yes, _, err := member.IsMemberOf("trainer")
	return yes, err
}

func ListTapingMenuItems(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	items := []models.TapingMenuItem{}
	query := datastore.NewQuery(models.KindTapingMenuItem).Order("SortOrder")
	if _, err := client.GetAll(ctx, query, &items); err != nil && !models.IsFiledMismatch(err) {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	for i, it := range items {
		items[i].ID = it.Key.ID
	}
	render.JSON(http.StatusOK, items)
}

func CreateTapingMenuItem(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	slackID := filters.GetSessionUserContext(req)
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	ok, err := isTapingManager(ctx, slackID, client)
	if err != nil || !ok {
		render.JSON(http.StatusForbidden, marmoset.P{"error": "forbidden"})
		return
	}

	item := models.TapingMenuItem{}
	defer req.Body.Close()
	if err := json.NewDecoder(req.Body).Decode(&item); err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}
	key, err := client.Put(ctx, datastore.IncompleteKey(models.KindTapingMenuItem, nil), &item)
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	item.ID = key.ID
	render.JSON(http.StatusCreated, item)
}

func UpdateTapingMenuItem(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	slackID := filters.GetSessionUserContext(req)
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	ok, err := isTapingManager(ctx, slackID, client)
	if err != nil || !ok {
		render.JSON(http.StatusForbidden, marmoset.P{"error": "forbidden"})
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}
	item := models.TapingMenuItem{}
	defer req.Body.Close()
	if err := json.NewDecoder(req.Body).Decode(&item); err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}
	key := datastore.IDKey(models.KindTapingMenuItem, id, nil)
	if _, err := client.Put(ctx, key, &item); err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	item.ID = id
	render.JSON(http.StatusOK, item)
}

func DeleteTapingMenuItem(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	slackID := filters.GetSessionUserContext(req)
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	ok, err := isTapingManager(ctx, slackID, client)
	if err != nil || !ok {
		render.JSON(http.StatusForbidden, marmoset.P{"error": "forbidden"})
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}
	if err := client.Delete(ctx, datastore.IDKey(models.KindTapingMenuItem, id, nil)); err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	render.JSON(http.StatusOK, marmoset.P{"id": id})
}

// SubmitTapingRequest は POST /api/1/taping/requests。
// body: { event_id: string, menu_item_ids: int64[] }
// 既存 Taping エンティティと diff して不要分を削除し、新規分を Put する。
func SubmitTapingRequest(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	slackID := filters.GetSessionUserContext(req)
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	body := struct {
		EventID     string  `json:"event_id"`
		MenuItemIDs []int64 `json:"menu_item_ids"`
	}{}
	defer req.Body.Close()
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}

	// メニューアイテムをまとめて取得してスナップショット用データを準備
	menuKeys := make([]*datastore.Key, len(body.MenuItemIDs))
	for i, mid := range body.MenuItemIDs {
		menuKeys[i] = datastore.IDKey(models.KindTapingMenuItem, mid, nil)
	}
	menuItems := make([]models.TapingMenuItem, len(body.MenuItemIDs))
	if len(menuKeys) > 0 {
		if err := client.GetMulti(ctx, menuKeys, menuItems); err != nil {
			if me, ok := err.(datastore.MultiError); ok {
				for _, e := range me {
					if e != nil && !models.IsFiledMismatch(e) {
						render.JSON(http.StatusInternalServerError, marmoset.P{"error": e.Error()})
						return
					}
				}
			} else {
				render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
				return
			}
		}
	}

	// 既存の Taping エンティティ（同 memberID + eventID）を取得
	existing := []models.Taping{}
	existQuery := datastore.NewQuery(models.KindTaping).
		Filter("MemberID =", slackID).
		Filter("EventID =", body.EventID)
	existKeys, err := client.GetAll(ctx, existQuery, &existing)
	if err != nil && !models.IsFiledMismatch(err) {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	// 新リクエストに含まれない既存エンティティを削除
	newSet := map[int64]bool{}
	for _, mid := range body.MenuItemIDs {
		newSet[mid] = true
	}
	toDelete := []*datastore.Key{}
	for i, t := range existing {
		if !newSet[t.MenuItemID] {
			toDelete = append(toDelete, existKeys[i])
		}
	}
	if len(toDelete) > 0 {
		if err := client.DeleteMulti(ctx, toDelete); err != nil {
			render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
			return
		}
	}

	// 新規・更新分を Put（NameKey により upsert）
	now := time.Now().Unix() * 1000
	result := []models.Taping{}
	for i, mid := range body.MenuItemIDs {
		item := menuItems[i]
		t := models.Taping{
			MemberID:       slackID,
			EventID:        body.EventID,
			MenuItemID:     mid,
			MenuItemName:   item.Name,
			Price:          item.Price,
			EstimatedRolls: item.EstimatedRolls,
			RequestedAt:    now,
		}
		nameKey := datastore.NameKey(models.KindTaping,
			fmt.Sprintf("%s_%s_%d", slackID, body.EventID, mid), nil)
		if _, err := client.Put(ctx, nameKey, &t); err != nil {
			render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
			return
		}
		result = append(result, t)
	}

	render.JSON(http.StatusOK, result)
}

func GetMyTapingRequest(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	slackID := filters.GetSessionUserContext(req)
	eventID := req.URL.Query().Get("event_id")
	if eventID == "" {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": "event_id is required"})
		return
	}
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	tapings := []models.Taping{}
	query := datastore.NewQuery(models.KindTaping).
		Filter("MemberID =", slackID).
		Filter("EventID =", eventID)
	if _, err := client.GetAll(ctx, query, &tapings); err != nil && !models.IsFiledMismatch(err) {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	render.JSON(http.StatusOK, tapings)
}

func ListTapingRequests(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	slackID := filters.GetSessionUserContext(req)
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	ok, err := isTapingManager(ctx, slackID, client)
	if err != nil || !ok {
		render.JSON(http.StatusForbidden, marmoset.P{"error": "forbidden"})
		return
	}

	query := datastore.NewQuery(models.KindTaping)
	if eventID := req.URL.Query().Get("event_id"); eventID != "" {
		query = query.Filter("EventID =", eventID)
	}
	tapings := []models.Taping{}
	if _, err := client.GetAll(ctx, query, &tapings); err != nil && !models.IsFiledMismatch(err) {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	render.JSON(http.StatusOK, tapings)
}

// ListTapingEvents は直近40日以内のイベントを新しい順で返す（テーピングリクエストのセレクト用）。
func ListTapingEvents(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	from := time.Now().Add(-40 * 24 * time.Hour).Unix() * 1000
	events := []models.Event{}
	query := datastore.NewQuery(models.KindEvent).
		Filter("Google.StartTime >", from).
		Order("Google.StartTime")
	if _, err := client.GetAll(ctx, query, &events); err != nil && !models.IsFiledMismatch(err) {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	render.JSON(http.StatusOK, events)
}
