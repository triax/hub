package fixtures

import (
	"testing"
	"time"

	"github.com/triax/hub/server/models"
)

var fixedNow = time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)

// 全 scenario が Validate を通過すること（受け入れ条件: go test ./fixtures/...）。
func TestRegisteredScenariosValidate(t *testing.T) {
	for _, name := range Names() {
		s, err := Resolve(fixedNow, name)
		if err != nil {
			t.Fatalf("Resolve(%q) failed: %v", name, err)
		}
		if err := Validate(s); err != nil {
			t.Errorf("scenario %q failed validation: %v", name, err)
		}
	}
}

func TestDefaultScenarioContents(t *testing.T) {
	s, err := Resolve(fixedNow, "default")
	if err != nil {
		t.Fatalf("Resolve(default): %v", err)
	}
	if len(s.Entities) == 0 {
		t.Fatal("default scenario is empty")
	}
	// admin Member が含まれること
	found := false
	for _, e := range s.Entities {
		if e.Key.Kind == models.KindMember && e.Key.Name == "U9MD7M0NS" {
			m := e.Value.(*models.Member)
			if !m.Slack.IsAdmin {
				t.Error("default admin Member is not IsAdmin")
			}
			found = true
		}
	}
	if !found {
		t.Error("default scenario missing admin Member U9MD7M0NS")
	}
}

// dangling key（参照先 entity 不在）を検出すること。
func TestValidateDetectsDanglingKey(t *testing.T) {
	taping := &models.Taping{
		MemberID:   "UNOBODY",  // 存在しない Member を参照
		EventID:    "no_event", // 存在しない Event を参照
		MenuItemID: 1,
	}
	s := Scenario{
		Name: "broken-dangling",
		Entities: []Entity{
			NewEntity(TapingKey("UNOBODY", "no_event", 1), taping),
		},
	}
	if err := Validate(s); err == nil {
		t.Fatal("expected dangling key error, got nil")
	}
}

// 未宣言の同一 key collision を検出すること。
func TestComposeDetectsCollision(t *testing.T) {
	m1 := &models.Member{}
	m1.Slack.ID = "UDUP"
	m1.Slack.RealName = "First"
	m2 := &models.Member{}
	m2.Slack.ID = "UDUP"
	m2.Slack.RealName = "Second" // 同一 key・異なる内容

	a := Scenario{Name: "a", Entities: []Entity{NewEntity(MemberKey("UDUP"), m1)}}
	b := Scenario{Name: "b", Entities: []Entity{NewEntity(MemberKey("UDUP"), m2)}}

	if _, err := Compose(a, b); err == nil {
		t.Fatal("expected collision error, got nil")
	}
}

// 同一内容なら dedup（衝突扱いしない）こと。
func TestComposeDedupsIdenticalKey(t *testing.T) {
	build := func() *models.Member {
		m := &models.Member{}
		m.Slack.ID = "USAME"
		m.Slack.RealName = "Same"
		return m
	}
	a := Scenario{Name: "a", Entities: []Entity{NewEntity(MemberKey("USAME"), build())}}
	b := Scenario{Name: "b", Entities: []Entity{NewEntity(MemberKey("USAME"), build())}}

	composed, err := Compose(a, b)
	if err != nil {
		t.Fatalf("identical key should dedup, got error: %v", err)
	}
	if len(composed.Entities) != 1 {
		t.Errorf("expected 1 deduped entity, got %d", len(composed.Entities))
	}
}

// Override 宣言があれば同一 key の上書きを許可すること。
func TestComposeAllowsDeclaredOverride(t *testing.T) {
	base := &models.Member{}
	base.Slack.ID = "UOVR"
	base.Slack.IsAdmin = false
	over := &models.Member{}
	over.Slack.ID = "UOVR"
	over.Slack.IsAdmin = true // 意図的に権限を足す

	a := Scenario{Name: "base", Entities: []Entity{NewEntity(MemberKey("UOVR"), base)}}
	b := Scenario{Name: "overlay", Entities: []Entity{Override(MemberKey("UOVR"), over)}}

	composed, err := Compose(a, b)
	if err != nil {
		t.Fatalf("declared Override should be allowed, got error: %v", err)
	}
	if len(composed.Entities) != 1 {
		t.Fatalf("expected 1 entity after override, got %d", len(composed.Entities))
	}
	got := composed.Entities[0].Value.(*models.Member)
	if !got.Slack.IsAdmin {
		t.Error("override did not take effect (expected IsAdmin=true)")
	}
}
