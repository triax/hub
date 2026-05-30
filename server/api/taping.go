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

// marshalTapeUsages は TapeUsages を JSON 文字列にシリアライズする。
func marshalTapeUsages(usages []models.TapeUsage) string {
	if len(usages) == 0 {
		return ""
	}
	b, _ := json.Marshal(usages)
	return string(b)
}

// unmarshalTapeUsages は JSON 文字列を TapeUsages にデシリアライズする。
func unmarshalTapeUsages(s string) []models.TapeUsage {
	if s == "" {
		return nil
	}
	var usages []models.TapeUsage
	json.Unmarshal([]byte(s), &usages)
	return usages
}

// --- TapeItem CRUD ---

func ListTapeItems(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	items := []models.TapeItem{}
	if _, err := client.GetAll(ctx, datastore.NewQuery(models.KindTapeItem).Order("SortOrder"), &items); err != nil && !models.IsFiledMismatch(err) {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	for i, it := range items {
		items[i].ID = it.Key.ID
	}
	render.JSON(http.StatusOK, items)
}

func CreateTapeItem(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	slackID := filters.GetSessionUserContext(req)
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	if ok, err := isTapingManager(ctx, slackID, client); err != nil || !ok {
		render.JSON(http.StatusForbidden, marmoset.P{"error": "forbidden"})
		return
	}

	item := models.TapeItem{}
	defer req.Body.Close()
	if err := json.NewDecoder(req.Body).Decode(&item); err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}
	key, err := client.Put(ctx, datastore.IncompleteKey(models.KindTapeItem, nil), &item)
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	item.ID = key.ID
	render.JSON(http.StatusCreated, item)
}

func UpdateTapeItem(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	slackID := filters.GetSessionUserContext(req)
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	if ok, err := isTapingManager(ctx, slackID, client); err != nil || !ok {
		render.JSON(http.StatusForbidden, marmoset.P{"error": "forbidden"})
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}
	item := models.TapeItem{}
	defer req.Body.Close()
	if err := json.NewDecoder(req.Body).Decode(&item); err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}
	if _, err := client.Put(ctx, datastore.IDKey(models.KindTapeItem, id, nil), &item); err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	item.ID = id
	render.JSON(http.StatusOK, item)
}

func DeleteTapeItem(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	slackID := filters.GetSessionUserContext(req)
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	if ok, err := isTapingManager(ctx, slackID, client); err != nil || !ok {
		render.JSON(http.StatusForbidden, marmoset.P{"error": "forbidden"})
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}
	if err := client.Delete(ctx, datastore.IDKey(models.KindTapeItem, id, nil)); err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	render.JSON(http.StatusOK, marmoset.P{"id": id})
}

// --- TapingMenuItem CRUD ---

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
	if _, err := client.GetAll(ctx, datastore.NewQuery(models.KindTapingMenuItem).Order("SortOrder"), &items); err != nil && !models.IsFiledMismatch(err) {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	for i, it := range items {
		items[i].ID = it.Key.ID
		items[i].TapeUsages = unmarshalTapeUsages(it.TapeUsagesJSON)
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

	if ok, err := isTapingManager(ctx, slackID, client); err != nil || !ok {
		render.JSON(http.StatusForbidden, marmoset.P{"error": "forbidden"})
		return
	}

	item := models.TapingMenuItem{}
	defer req.Body.Close()
	if err := json.NewDecoder(req.Body).Decode(&item); err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}
	item.TapeUsagesJSON = marshalTapeUsages(item.TapeUsages)
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

	if ok, err := isTapingManager(ctx, slackID, client); err != nil || !ok {
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
	item.TapeUsagesJSON = marshalTapeUsages(item.TapeUsages)
	if _, err := client.Put(ctx, datastore.IDKey(models.KindTapingMenuItem, id, nil), &item); err != nil {
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

	if ok, err := isTapingManager(ctx, slackID, client); err != nil || !ok {
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

// --- Taping requests ---

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
	existKeys, err := client.GetAll(ctx,
		datastore.NewQuery(models.KindTaping).Filter("MemberID =", slackID).Filter("EventID =", body.EventID),
		&existing,
	)
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

	// 新規・更新分を PutMulti（NameKey により upsert）
	now := time.Now().Unix() * 1000
	putKeys := make([]*datastore.Key, 0, len(body.MenuItemIDs))
	putValues := make([]*models.Taping, 0, len(body.MenuItemIDs))
	for i, mid := range body.MenuItemIDs {
		menuItem := menuItems[i]
		t := &models.Taping{
			MemberID:       slackID,
			EventID:        body.EventID,
			MenuItemID:     mid,
			MenuItemName:   menuItem.Name,
			Price:          menuItem.Price,
			TapeUsagesJSON: menuItem.TapeUsagesJSON, // JSON 文字列をそのままコピー（decode→encodeの往復を省略）
			TapeUsages:     unmarshalTapeUsages(menuItem.TapeUsagesJSON),
			RequestedAt:    now,
		}
		putKeys = append(putKeys, datastore.NameKey(models.KindTaping,
			fmt.Sprintf("%s_%s_%d", slackID, body.EventID, mid), nil))
		putValues = append(putValues, t)
	}
	if len(putKeys) > 0 {
		if _, err := client.PutMulti(ctx, putKeys, putValues); err != nil {
			render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
			return
		}
	}

	result := make([]models.Taping, len(putValues))
	for i, t := range putValues {
		result[i] = *t
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
	if _, err := client.GetAll(ctx,
		datastore.NewQuery(models.KindTaping).Filter("MemberID =", slackID).Filter("EventID =", eventID),
		&tapings,
	); err != nil && !models.IsFiledMismatch(err) {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	for i, t := range tapings {
		tapings[i].TapeUsages = unmarshalTapeUsages(t.TapeUsagesJSON)
	}
	render.JSON(http.StatusOK, tapings)
}

func ListTapingRequests(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	query := datastore.NewQuery(models.KindTaping)
	if eventID := req.URL.Query().Get("event_id"); eventID != "" {
		query = query.Filter("EventID =", eventID)
	}
	if yearStr := req.URL.Query().Get("year"); yearStr != "" {
		if y, err := strconv.Atoi(yearStr); err == nil {
			loc := time.FixedZone("Asia/Tokyo", 9*60*60)
			from := time.Date(y, 1, 1, 0, 0, 0, 0, loc).UnixMilli()
			to := time.Date(y+1, 1, 1, 0, 0, 0, 0, loc).UnixMilli()
			query = query.Filter("RequestedAt >=", from).Filter("RequestedAt <", to)
		}
	}
	tapings := []models.Taping{}
	if _, err := client.GetAll(ctx, query, &tapings); err != nil && !models.IsFiledMismatch(err) {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	for i, t := range tapings {
		tapings[i].TapeUsages = unmarshalTapeUsages(t.TapeUsagesJSON)
	}
	render.JSON(http.StatusOK, tapings)
}

// ListTapingEvents は直近40日以内のイベントを返す（テーピングリクエストのセレクト用）。
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
	if _, err := client.GetAll(ctx,
		datastore.NewQuery(models.KindEvent).Filter("Google.StartTime >", from).Order("Google.StartTime"),
		&events,
	); err != nil && !models.IsFiledMismatch(err) {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	render.JSON(http.StatusOK, events)
}
