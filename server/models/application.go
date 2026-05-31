package models

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/datastore"
)

type ApplicationStep struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Done  bool   `json:"done"`
}

type Application struct {
	Type            string            `json:"type"`
	Email           string            `json:"email"`
	Name            string            `json:"name"`
	Fields          map[string]string `json:"fields" datastore:"-"`
	FieldsJSON      string            `json:"-" datastore:"fields_json,noindex"`
	ConsentAgreedAt time.Time         `json:"consent_agreed_at"`
	Steps           []ApplicationStep `json:"steps" datastore:",noindex"`
	Done            bool              `json:"done"`
	CreatedAt       time.Time         `json:"created_at"`
}

func (a *Application) MarshalFields() error {
	if a.Fields == nil {
		a.FieldsJSON = "{}"
		return nil
	}
	b, err := json.Marshal(a.Fields)
	if err != nil {
		return err
	}
	a.FieldsJSON = string(b)
	return nil
}

func (a *Application) UnmarshalFields() error {
	a.Fields = map[string]string{}
	if a.FieldsJSON == "" {
		return nil
	}
	return json.Unmarshal([]byte(a.FieldsJSON), &a.Fields)
}

func GetApplication(ctx context.Context, id string) (*Application, error) {
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		return nil, fmt.Errorf("datastore client: %w", err)
	}
	defer client.Close()

	app := &Application{}
	key := datastore.NameKey(KindApplication, id, nil)
	if err := client.Get(ctx, key, app); err != nil {
		if err == datastore.ErrNoSuchEntity {
			return nil, nil
		}
		if IsFiledMismatch(err) {
			_ = app.UnmarshalFields()
			return app, nil
		}
		return nil, fmt.Errorf("datastore Get: %w", err)
	}
	_ = app.UnmarshalFields()
	return app, nil
}

func PutApplication(ctx context.Context, id string, app *Application) error {
	if err := app.MarshalFields(); err != nil {
		return fmt.Errorf("marshal fields: %w", err)
	}
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		return fmt.Errorf("datastore client: %w", err)
	}
	defer client.Close()

	key := datastore.NameKey(KindApplication, id, nil)
	if _, err := client.Put(ctx, key, app); err != nil {
		return fmt.Errorf("datastore Put: %w", err)
	}
	return nil
}

func ListApplications(ctx context.Context, appType string) ([]*Application, []string, error) {
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		return nil, nil, fmt.Errorf("datastore client: %w", err)
	}
	defer client.Close()

	q := datastore.NewQuery(KindApplication).Order("-CreatedAt")
	if appType != "" {
		q = q.FilterField("Type", "=", appType)
	}

	var apps []*Application
	keys, err := client.GetAll(ctx, q, &apps)
	if err != nil && !IsFiledMismatch(err) {
		return nil, nil, fmt.Errorf("datastore GetAll: %w", err)
	}
	ids := make([]string, len(keys))
	for i, k := range keys {
		ids[i] = k.Name
		_ = apps[i].UnmarshalFields()
	}
	return apps, ids, nil
}
