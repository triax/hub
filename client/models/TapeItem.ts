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

  // TapingMenuItem.sort と同じ方針（日本語名の昇順・sortOrder は使わない）。
  static sort(p: TapeItem, n: TapeItem): number {
    return p.name.localeCompare(n.name, "ja");
  }

  static listFromAPIResponse(res: any[]): TapeItem[] {
    return res.map(TapeItem.fromAPIResponse).sort(TapeItem.sort);
  }
}
