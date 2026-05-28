package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/go-chi/chi/v5"
	"github.com/otiai10/marmoset"
	"github.com/triax/hub/server/filters"
	"github.com/triax/hub/server/models"
)

var allowedMIMETypes = map[string]string{
	"image/png":  "png",
	"image/jpeg": "jpg",
	"image/gif":  "gif",
	"image/webp": "webp",
}

const maxPhotoBytes = 10 << 20 // 10MB

func GetHPProfile(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	id := chi.URLParam(req, "id")
	if filters.GetSessionUserContext(req) != id {
		render.JSON(http.StatusForbidden, marmoset.P{"error": "forbidden"})
		return
	}
	profile, err := models.GetHPProfile(req.Context(), id)
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	render.JSON(http.StatusOK, profile)
}

func UpdateHPProfile(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	id := chi.URLParam(req, "id")

	callerID := filters.GetSessionUserContext(req)
	if callerID != id {
		render.JSON(http.StatusForbidden, marmoset.P{"error": "forbidden"})
		return
	}

	var input models.MemberHPProfile
	if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}

	// 既存プロフィールを取得して写真 URL を保持する（PUT で消えないように）
	existing, err := models.GetHPProfile(req.Context(), id)
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	if input.PortraitFormalURL == "" {
		input.PortraitFormalURL = existing.PortraitFormalURL
	}
	if input.PortraitCasualURL == "" {
		input.PortraitCasualURL = existing.PortraitCasualURL
	}
	// 追加写真の URL は専用の photo エンドポイント経由でのみ変更可能。
	// PUT ボディに含まれる値（nil でも [] でも）は常に無視して既存値を保持する。
	input.AdditionalPhotoURLs = existing.AdditionalPhotoURLs

	if err := models.PutHPProfile(req.Context(), id, &input); err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	render.JSON(http.StatusOK, input)
}

// UploadHPPhoto は写真を GCS にアップロードし、公開 URL を返す。
// URL パラメータ: ?type=formal|casual|additional
func UploadHPPhoto(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	id := chi.URLParam(req, "id")

	callerID := filters.GetSessionUserContext(req)
	if callerID != id {
		render.JSON(http.StatusForbidden, marmoset.P{"error": "forbidden"})
		return
	}

	photoType := req.URL.Query().Get("type")
	if photoType != "formal" && photoType != "casual" && photoType != "additional" {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": "type must be formal, casual, or additional"})
		return
	}

	if !strings.HasPrefix(req.Header.Get("Content-Type"), "multipart/form-data") {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": "multipart/form-data required"})
		return
	}
	if err := req.ParseMultipartForm(maxPhotoBytes); err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": "failed to parse multipart form"})
		return
	}
	file, header, err := req.FormFile("photo")
	if err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": "photo field required"})
		return
	}
	defer file.Close()
	if header.Size > maxPhotoBytes {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": "file too large (max 10MB)"})
		return
	}
	detectedMIME := header.Header.Get("Content-Type")
	var fileData io.Reader = file

	ext, ok := allowedMIMETypes[detectedMIME]
	if !ok {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": fmt.Sprintf("unsupported image type: %s", detectedMIME)})
		return
	}

	objectName := fmt.Sprintf("hp-profile/%s/%s-%d.%s", id, photoType, time.Now().UnixMilli(), ext)
	publicURL, err := uploadToGCS(req.Context(), objectName, detectedMIME, fileData)
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	// プロフィールを更新して URL を保存
	profile, err := models.GetHPProfile(req.Context(), id)
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	switch photoType {
	case "formal":
		profile.PortraitFormalURL = publicURL
	case "casual":
		profile.PortraitCasualURL = publicURL
	case "additional":
		profile.AdditionalPhotoURLs = append(profile.AdditionalPhotoURLs, publicURL)
	}
	if err := models.PutHPProfile(req.Context(), id, profile); err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	render.JSON(http.StatusOK, marmoset.P{"url": publicURL})
}

func uploadToGCS(ctx context.Context, objectName, mimeType string, r io.Reader) (string, error) {
	bucketName := os.Getenv("GCS_HP_PHOTO_BUCKET")
	if bucketName == "" {
		return "", fmt.Errorf("GCS_HP_PHOTO_BUCKET is not set")
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		return "", fmt.Errorf("storage.NewClient: %w", err)
	}
	defer client.Close()

	obj := client.Bucket(bucketName).Object(objectName)
	wc := obj.NewWriter(ctx)
	wc.ContentType = mimeType
	// 公開アクセスはバケットレベルの IAM (roles/storage.objectViewer → allUsers) で設定すること。
	// PredefinedACL = "publicRead" は Uniform bucket-level access が有効な場合に失敗するため使用しない。

	if _, err := io.Copy(wc, r); err != nil {
		return "", fmt.Errorf("io.Copy to GCS: %w", err)
	}
	if err := wc.Close(); err != nil {
		return "", fmt.Errorf("GCS writer Close: %w", err)
	}

	return fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucketName, objectName), nil
}

// ListPublicMembers は認証不要の公開 API。
// HideFromHP=false のメンバーのみ返し、HiddenFields に従ってフィールドを除外する。
func ListPublicMembers(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()

	members, err := models.GetAllMembers(ctx)
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	type publicEntry struct {
		SlackID   string                 `json:"slack_id"`
		Name      string                 `json:"name"`
		Number    *int                   `json:"number"`
		HPProfile models.MemberHPProfile `json:"hp_profile"`
	}

	profiles, err := models.GetMultiHPProfile(ctx, members)
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	result := make([]publicEntry, 0, len(members))
	for i, m := range members {
		profile := profiles[i]
		if profile == nil || profile.HideFromHP {
			continue
		}
		result = append(result, publicEntry{
			SlackID:   m.Slack.ID,
			Name:      m.Name(),
			Number:    m.Number,
			HPProfile: profile.PublicView(),
		})
	}

	// 30 分キャッシュ（外部サイト向け）
	w.Header().Set("Cache-Control", "public, max-age=1800")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	render.JSON(http.StatusOK, marmoset.P{
		"members": result,
		"path":    path.Clean(req.URL.Path),
	})
}
