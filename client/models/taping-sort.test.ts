import { describe, expect, it } from "vitest";
import TapeItem from "./TapeItem";
import Taping from "./Taping";
import TapingMenuItem from "./TapingMenuItem";

// 日本語名の昇順は localeCompare(_, "ja") が定める照合順に従う。
// 期待値をハードコードすると ICU の版差で脆くなるため、
// 「隣接ペアが単調非減少であること」＋「カナの相対順序」で検証する。
const isAscending = (names: string[]): boolean =>
  names.every((_, i) => i === 0 || names[i - 1].localeCompare(names[i], "ja") <= 0);

const menuItem = (id: number, name: string): TapingMenuItem =>
  TapingMenuItem.fromAPIResponse({
    id, name, price: 0, notes: "", tape_usages: [], sort_order: 0, disabled: false,
  });

const tapeItem = (id: number, name: string): TapeItem =>
  TapeItem.fromAPIResponse({ id, name, stock_count: 0, sort_order: 0, disabled: false });

const taping = (menuItemID: number, menuItemName: string, note = ""): Taping =>
  Taping.fromAPIResponse({
    member_id: "U1", event_id: "E1", menu_item_id: menuItemID, menu_item_name: menuItemName,
    price: 0, tape_usages: [], note, requested_at: 0,
  });

describe("TapingMenuItem.sort", () => {
  // ひらがな・カタカナ・漢字が混在し、かつ ID 順（登録順）と名前順が一致しない集合。
  const names = ["膝", "足首", "肩", "アキレス腱", "ふくらはぎ"];

  it("日本語名の昇順で並び、混在しても例外を投げない", () => {
    const sorted = names.map((n, i) => menuItem(9001 + i, n)).sort(TapingMenuItem.sort);
    expect(isAscending(sorted.map(it => it.name))).toBe(true);
    expect(sorted).toHaveLength(names.length);
  });

  it("listFromAPIResponse がソート済み配列を返す（choke point）", () => {
    const res = names.map((name, i) => ({
      id: 9001 + i, name, price: 0, notes: "", tape_usages: [], sort_order: 0, disabled: false,
    }));
    expect(isAscending(TapingMenuItem.listFromAPIResponse(res).map(it => it.name))).toBe(true);
  });

  it("sort_order は比較に使わない（既定方針: 名前順で全画面を揃える）", () => {
    const [a, b] = [menuItem(1, "アキレス腱"), menuItem(2, "膝")];
    a.sortOrder = 999;
    b.sortOrder = 0;
    expect([b, a].sort(TapingMenuItem.sort).map(it => it.name)).toEqual(["アキレス腱", "膝"]);
  });

  it("カナは五十音順に並ぶ", () => {
    const sorted = [menuItem(1, "ふくらはぎ"), menuItem(2, "アキレス腱")].sort(TapingMenuItem.sort);
    expect(sorted.map(it => it.name)).toEqual(["アキレス腱", "ふくらはぎ"]);
  });
});

describe("TapeItem.sort", () => {
  it("日本語名の昇順で並ぶ", () => {
    const res = [
      { id: 9103, name: "ホワイトテープ", stock_count: 0, sort_order: 0, disabled: false },
      { id: 9101, name: "キネシオテープ", stock_count: 0, sort_order: 0, disabled: false },
      { id: 9102, name: "アンダーラップ", stock_count: 0, sort_order: 0, disabled: false },
    ];
    const sorted = TapeItem.listFromAPIResponse(res).map(it => it.name);
    expect(isAscending(sorted)).toBe(true);
    expect(sorted[0]).toBe("アンダーラップ");
  });

  it("空配列でも例外を投げない", () => {
    expect(TapeItem.listFromAPIResponse([])).toEqual([]);
    expect([tapeItem(1, "テープ")].sort(TapeItem.sort)).toHaveLength(1);
  });
});

describe("Taping.sort", () => {
  it("施術メニューは日本語名の昇順で並ぶ", () => {
    const sorted = [taping(3, "膝"), taping(1, "アキレス腱"), taping(2, "ふくらはぎ")]
      .sort(Taping.sort);
    expect(isAscending(sorted.map(t => t.menuItemName))).toBe(true);
  });

  it("「その他」（menuItemID === 0）は名前に関わらず常に末尾に置かれる", () => {
    const sorted = [
      taping(0, "その他", "右手首をぐるっと巻いてほしい"),
      taping(3, "膝"),
      taping(1, "アキレス腱"),
    ].sort(Taping.sort);
    expect(sorted[sorted.length - 1].menuItemID).toBe(0);
    expect(sorted[sorted.length - 1].note).toBe("右手首をぐるっと巻いてほしい");
    // 「そ」の位置（アキレス腱と膝の間）に紛れ込んでいないこと
    expect(sorted.slice(0, -1).every(t => !Taping.isNote(t))).toBe(true);
  });

  it("listFromAPIResponse でも「その他」が末尾に来る", () => {
    const res = [
      { member_id: "U1", event_id: "E1", menu_item_id: 0, menu_item_name: "その他", price: 0, tape_usages: [], note: "テープの色を白で", requested_at: 0 },
      { member_id: "U1", event_id: "E1", menu_item_id: 9, menu_item_name: "肩", price: 100, tape_usages: [], note: "", requested_at: 0 },
    ];
    const sorted = Taping.listFromAPIResponse(res);
    expect(sorted.map(t => t.menuItemID)).toEqual([9, 0]);
    expect(sorted[1].note).toBe("テープの色を白で");
  });

  it("note が未指定のレスポンスでも空文字にフォールバックする（後方互換）", () => {
    expect(taping(1, "膝").note).toBe("");
    expect(Taping.fromAPIResponse({
      member_id: "U1", event_id: "E1", menu_item_id: 1, menu_item_name: "膝",
      price: 0, tape_usages: [], note: undefined, requested_at: 0,
    }).note).toBe("");
  });
});
