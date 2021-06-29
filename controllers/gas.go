package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"cloud.google.com/go/datastore"
	"github.com/triax/hub/models"
)

func SyncCalendarEvetns(w http.ResponseWriter, req *http.Request) {
	if req.Header.Get("X-Hub-Verifier") != os.Getenv("GAS_ACCESS_VERIFIER") {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	payload := struct {
		Events []models.GoogleEvent `json:"events"`
	}{}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		fmt.Println("[ERROR]", 6001, err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	keys := []*datastore.Key{}
	events := []models.Event{}
	for _, event := range payload.Events {
		keys = append(keys, datastore.NameKey(models.KindEvent, event.ID, nil))
		events = append(events, models.Event{Google: event})
	}

	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		fmt.Println("[ERROR]", 6002, err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if _, err := client.PutMulti(ctx, keys, events); err != nil {
		fmt.Println("[ERROR]", 6003, err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	fmt.Printf("%+v\n", payload.Events)
	w.WriteHeader(http.StatusOK)
}
