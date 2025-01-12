package models

type Decoration string

const (
	Deco_REFINVERSE_PATCH_v2024 = "refinverse_patch_v2024"
)

var (
	DefaultDecorations = map[Decoration]bool{
		Deco_REFINVERSE_PATCH_v2024: false,
	}
)

/**
 * 背番号という概念
 */
type PlayerNumber struct {

	// 背番号の値
	Number int `json:"number"`

	// 背番号は、0から複数のユニフォームが紐づく
	Uniforms []Uniform `json:"uniforms"`

	// 割り当てられた選手の Slack ID
	PlayerID string `json:"player_id"`

	// -- Populated fields --
	Player Member `json:"player,omitempty"`
}

/**
 * ユニフォームの概念
 */
type Uniform struct {
	// ユニフォームに印字されている番号
	Number uint `json:"number"`

	// ユニフォームのサイズ [S, M, L, XL, XXL]
	Size string `json:"size"`

	// ユニフォームの色
	Color bool `json:"color"` // true: 赤, false: 白

	// ユニフォームの状態 [true = 難あり, false = 問題なし] の2パターンのみ
	Damaged bool `json:"damaged"`

	// ユニフォームに付与されたデコレーション
	Decoration map[Decoration]bool `json:"decoration,omitempty"`

	// ユニフォームの所有者の Slack ID (空の場合、所有者はチームである)
	OwnerID string `json:"owner_id,omitempty"`

	// -- Populated fields --
	Owner Member `json:"owner,omitempty"`
}
