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

  static listFromAPIResponse(res: any[]): TapeItem[] {
    return res.map(TapeItem.fromAPIResponse);
  }
}
