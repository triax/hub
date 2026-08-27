package api

import (
	"strings"
	"testing"
)

// TestMemberCacheControl_NotImmutable は、メンバー情報レスポンスの Cache-Control に
// immutable が付かないことを固定する（Issue #536）。
//
// immutable はリロードしても再検証させないディレクティブなので、これが付いていると
// Slack プロフィールのポジション（Title）を変更しても、ユーザは最大 max-age の間
// 反映させる手段を持てない。max-age 自体は Datastore 読み取りコスト削減のために残す。
func TestMemberCacheControl_NotImmutable(t *testing.T) {
	got := memberCacheControl(memberCacheMaxAgeSeconds)

	if strings.Contains(got, "immutable") {
		t.Fatalf("Cache-Control must not contain %q, got %q", "immutable", got)
	}
	if want := "max-age=7200"; !strings.Contains(got, want) {
		t.Fatalf("Cache-Control must contain %q (Datastore 読み取りコスト削減), got %q", want, got)
	}
	if want := "public"; !strings.Contains(got, want) {
		t.Fatalf("Cache-Control must contain %q, got %q", want, got)
	}
}

// TestMemberCacheControl_MaxAgeReflectsArgument は、max-age が引数で決まることを検証する。
func TestMemberCacheControl_MaxAgeReflectsArgument(t *testing.T) {
	if got, want := memberCacheControl(60), "public, max-age=60"; got != want {
		t.Fatalf("memberCacheControl(60) = %q, want %q", got, want)
	}
}
