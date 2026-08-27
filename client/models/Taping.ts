import { TapeUsage } from "./TapingMenuItem";

export default class Taping {
  constructor(
    public memberID: string,
    public eventID: string,
    public menuItemID: number,
    public menuItemName: string,
    public price: number,
    public tapeUsages: TapeUsage[],
    public note: string,
    public requestedAt: number,
  ) {}

  // menuItemID === 0 は「その他」自由記述のエンティティ（対応するマスタ項目を持たない）。
  static isNote(t: Taping): boolean {
    return t.menuItemID === 0;
  }

  static fromAPIResponse({ member_id, event_id, menu_item_id, menu_item_name, price, tape_usages, note, requested_at }): Taping {
    return new Taping(
      member_id ?? "",
      event_id ?? "",
      menu_item_id ?? 0,
      menu_item_name ?? "",
      price ?? 0,
      tape_usages ?? [],
      note ?? "",
      requested_at ?? 0,
    );
  }

  // 施術メニューは日本語名の昇順。「その他」自由記述は名前順に紛れ込ませず常に末尾に置く
  // （/taping/request 側でチェックボックス群の末尾に固定されているのと同じ並びにする）。
  static sort(p: Taping, n: Taping): number {
    if (Taping.isNote(p) || Taping.isNote(n)) {
      return Number(Taping.isNote(p)) - Number(Taping.isNote(n));
    }
    return p.menuItemName.localeCompare(n.menuItemName, "ja");
  }

  // 表示順のソートはこの choke point に集約する（ページ側の呼び出し漏れを防ぐ）。
  static listFromAPIResponse(res: any[]): Taping[] {
    return res.map(Taping.fromAPIResponse).sort(Taping.sort);
  }
}
