package slackbot

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"

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
// ゼロ値で利用でき、構築時にネットワークへ触れない（サービスは Enqueue 内で遅延生成）。
type CloudTasksEnqueuer struct{}

// 環境変数名。CLOUD_TASKS_LOCATION は必須（未設定なら enqueue はエラー）。
const (
	envCloudTasksLocation  = "CLOUD_TASKS_LOCATION"
	envCloudTasksQueue     = "CLOUD_TASKS_QUEUE"
	defaultCloudTasksQueue = "focus"
)

func (CloudTasksEnqueuer) Enqueue(ctx context.Context, name, relativeURI string, payload []byte) error {
	project := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if project == "" {
		return fmt.Errorf("GOOGLE_CLOUD_PROJECT is not set")
	}
	location := os.Getenv(envCloudTasksLocation)
	if location == "" {
		return fmt.Errorf("%s is not set (例: asia-northeast1)", envCloudTasksLocation)
	}
	queue := os.Getenv(envCloudTasksQueue)
	if queue == "" {
		queue = defaultCloudTasksQueue
	}

	// 認証は App Engine ランタイムの既定サービスアカウント（ADC）。
	service, err := cloudtasks.NewService(ctx)
	if err != nil {
		return fmt.Errorf("cloudtasks.NewService: %w", err)
	}

	parent := fmt.Sprintf("projects/%s/locations/%s/queues/%s", project, location, queue)
	req := &cloudtasks.CreateTaskRequest{Task: &cloudtasks.Task{
		Name: parent + "/tasks/" + name,
		AppEngineHttpRequest: &cloudtasks.AppEngineHttpRequest{
			HttpMethod:  http.MethodPost,
			RelativeUri: relativeURI,
			Headers:     map[string]string{"Content-Type": "application/json"},
			// REST API の bytes フィールドなので base64 で渡す。
			Body: base64.StdEncoding.EncodeToString(payload),
			// GAE_SERVICE を明示しないと dev サービスからの enqueue が
			// default サービスへ配送されてしまう。
			AppEngineRouting: &cloudtasks.AppEngineRouting{Service: os.Getenv("GAE_SERVICE")},
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
