export default class TapeItem {
  constructor(
    public id: number,
    public name: string,
    public stockCount: number,
    public sortOrder: number,
    public disabled: boolean,
  ) {}

  static fromAPIResponse({ id, name, stock_count, sort_order, disabled }): TapeItem {
    return new TapeItem(id ?? 0, name ?? "", stock_count ?? 0, sort_order ?? 0, disabled ?? false);
  }

  // 日本語名の昇順コンパレータ（TapingMenuItem と同じ方針。sortOrder は使わない）。
  static sort(p: TapeItem, n: TapeItem): number {
    return p.name.localeCompare(n.name, "ja");
  }

  // 表示順のソートはこの choke point に集約する（ページ側の呼び出し漏れを防ぐ）。
  static listFromAPIResponse(res: any[]): TapeItem[] {
    return res.map(TapeItem.fromAPIResponse).sort(TapeItem.sort);
  }
}
