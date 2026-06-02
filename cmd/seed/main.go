// Command seed は fixtures の scenario を Datastore エミュレーターへ投入する。
//
// 使い方:
//
//	go run ./cmd/seed --scenarios default
//	go run ./cmd/seed --validate-only --scenarios default
//
// 環境変数 DATASTORE_EMULATOR_HOST が設定されていれば、datastore client は
// 自動的にエミュレーターへ接続する。project ID は --project / DATASTORE_PROJECT_ID /
// GOOGLE_CLOUD_PROJECT の順で解決する。
//
// emulator 接続失敗 / validation 失敗 / upsert 失敗のいずれでも非 0 終了する。
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/triax/hub/fixtures"
)

func main() {
	var (
		scenariosFlag = flag.String("scenarios", "default", "投入する scenario 名（カンマ区切り）")
		validateOnly  = flag.Bool("validate-only", false, "Datastore へ投入せず validation のみ実行する")
		projectFlag   = flag.String("project", "", "Datastore project ID（未指定時は env から解決）")
	)
	flag.Parse()

	if err := run(*scenariosFlag, *validateOnly, *projectFlag); err != nil {
		log.Fatalf("seed: %v", err)
	}
}

func run(scenariosCSV string, validateOnly bool, projectFlag string) error {
	names := splitCSV(scenariosCSV)
	if len(names) == 0 {
		return fmt.Errorf("no scenarios specified")
	}

	// 相対日付を含む scenario のため now を 1 度だけ確定する。
	scenario, err := fixtures.Resolve(time.Now(), names...)
	if err != nil {
		return err
	}

	if err := fixtures.Validate(scenario); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if validateOnly {
		fmt.Printf("validated %d entities from scenarios %v\n", len(scenario.Entities), names)
		return nil
	}

	projectID := resolveProjectID(projectFlag)
	if projectID == "" {
		return fmt.Errorf("project ID unresolved: set --project, DATASTORE_PROJECT_ID, or GOOGLE_CLOUD_PROJECT")
	}

	ctx := context.Background()
	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		return fmt.Errorf("datastore client: %w", err)
	}
	defer client.Close()

	if err := fixtures.Load(ctx, client, scenario); err != nil {
		return err
	}

	fmt.Printf("seeded %d entities from scenarios %v into project %q\n", len(scenario.Entities), names, projectID)
	return nil
}

func resolveProjectID(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	if v := os.Getenv("DATASTORE_PROJECT_ID"); v != "" {
		return v
	}
	return os.Getenv("GOOGLE_CLOUD_PROJECT")
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
