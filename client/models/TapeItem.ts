export default class TapeItem {
  constructor(
    public id: number,
    public name: string,
    public sortOrder: number,
    public disabled: boolean,
  ) {}

  static fromAPIResponse({ id, name, sort_order, disabled }): TapeItem {
    return new TapeItem(id ?? 0, name ?? "", sort_order ?? 0, disabled ?? false);
  }

  static listFromAPIResponse(res: any[]): TapeItem[] {
    return res.map(TapeItem.fromAPIResponse);
  }
}
