package slackbot

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"

	// stellar:debt(dep) 既存依存 google.golang.org/api に同梱の REST クライアントを使い、
	// cloud.google.com/go/cloudtasks の追加を避けている（この生成パッケージは
	// deprecated 表記だが v2 API として現役）。upgrade: gRPC 版 cloudtasks/apiv2 へ移行
	cloudtasks "google.golang.org/api/cloudtasks/v2"
	"google.golang.org/api/googleapi"
)

// TaskEnqueuer は「重い処理をリクエストの外へ逃がす」ためのキュー抽象。
// テストでは fake を差し込み、ローカル開発では nil にしてインプロセス実行へ倒す。
type TaskEnqueuer interface {
	// Enqueue は name を task ID とするタスクを積む。
	// 同じ name のタスクが既に存在する場合は成功として扱う（二重 enqueue の抑止）。
	Enqueue(ctx context.Context, name, relativeURI string, payload []byte) error
}

// CloudTasksEnqueuer は Cloud Tasks（App Engine ターゲット）へタスクを積む。
// 設定は呼び出し側（main.go）が環境から解決して渡す。この層は環境変数を読まない。
// 構築時にネットワークへは触れない（サービスは Enqueue 内で遅延生成）。
type CloudTasksEnqueuer struct {
	Project  string // GOOGLE_CLOUD_PROJECT
	Location string // 例: asia-northeast1
	Queue    string // 例: focus
	// Service は配送先の App Engine サービス（GAE_SERVICE）。空なら default サービスへ。
	// 明示しないと dev サービスからの enqueue が default へ飛ぶ。
	Service string
}

// DefaultCloudTasksQueue は CLOUD_TASKS_QUEUE 未設定時に使うキュー名。
const DefaultCloudTasksQueue = "focus"

func (e CloudTasksEnqueuer) Enqueue(ctx context.Context, name, relativeURI string, payload []byte) error {
	if e.Project == "" {
		return fmt.Errorf("GOOGLE_CLOUD_PROJECT が未設定のため focus を実行できません")
	}
	if e.Location == "" {
		return fmt.Errorf("CLOUD_TASKS_LOCATION が未設定のため focus を実行できません（例: asia-northeast1）")
	}
	queue := e.Queue
	if queue == "" {
		queue = DefaultCloudTasksQueue
	}

	// 認証は App Engine ランタイムの既定サービスアカウント（ADC）。
	service, err := cloudtasks.NewService(ctx)
	if err != nil {
		return fmt.Errorf("cloudtasks.NewService: %w", err)
	}

	parent := fmt.Sprintf("projects/%s/locations/%s/queues/%s", e.Project, e.Location, queue)
	req := &cloudtasks.CreateTaskRequest{Task: &cloudtasks.Task{
		Name: parent + "/tasks/" + name,
		AppEngineHttpRequest: &cloudtasks.AppEngineHttpRequest{
			HttpMethod:  http.MethodPost,
			RelativeUri: relativeURI,
			Headers:     map[string]string{"Content-Type": "application/json"},
			// REST API の bytes フィールドなので base64 で渡す。
			Body:             base64.StdEncoding.EncodeToString(payload),
			AppEngineRouting: &cloudtasks.AppEngineRouting{Service: e.Service},
		},
	}}

	if _, err := service.Projects.Locations.Queues.Tasks.Create(parent, req).Context(ctx).Do(); err != nil {
		// 同じ task ID が既にある = Slack の再送等による二重 enqueue。正常系として無視する。
		var gerr *googleapi.Error
		if errors.As(err, &gerr) && gerr.Code == http.StatusConflict {
			return nil
		}
		return fmt.Errorf("cloudtasks.Create: %w", err)
	}
	return nil
}
