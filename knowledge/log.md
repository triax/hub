# Change Log

## 2026-05-30

### テーピング管理機能を追加 (PR #571, #572)

**概要:** メンバーのテーピングリクエスト申請・集計・在庫管理機能をフルスタックで実装。

**新規ページ:**
- `/taping/request` — 部位チェックボックス＋イベントセレクトで申請（全メンバー）
- `/taping` — 年度費用集計・テープ在庫状況（全メンバー）
- `/taping/master` — 施術メニュー＆テープ素材のマスタ管理（Admin/Trainer/Staff）
- `/events/{id}/taping` — イベント別リクエスト一覧（全メンバー）

**既存画面の変更:**
- `/events/{id}` — タイトル横に「テーピング確認」ボタンを追加
- ナビゲーション — `Uniforms` → `Taping`（`/taping`）に変更

**データモデル:**
- `TapeItem` — テープ素材マスタ（name, stockCount, sortOrder, disabled）
- `TapingMenuItem` — 施術メニューマスタ（name, price, tapeUsages[], notes, sortOrder, disabled）
- `Taping` — リクエスト記録 per-item型（memberID, eventID, menuItemID, menuItemName, price, tapeUsages[], requestedAt）

**設計の特徴:**
- `Taping` は NameKey(`memberID_eventID_menuItemID`) により `PutMulti` で upsert
- 申請時点の price/tapeUsages をスナップショット保存（後からのマスタ変更が過去請求に影響しない）
- テープ在庫状況は今後のイベント申請を集計した推定必要量と基本ストック本数を比較

**マニュアル:** [manual/taping.html](manual/taping.html)
