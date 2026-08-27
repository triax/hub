export interface TapeUsage {
  tape_item_id: number;
  tape_item_name: string;
  quantity: number;
}

export interface TapingMenuItemDraft {
  name: string;
  price: number;
  notes: string;
  tape_usages: TapeUsage[];
  sort_order: number;
  disabled: boolean;
}

export default class TapingMenuItem {
  constructor(
    public id: number,
    public name: string,
    public price: number,
    public notes: string,
    public tapeUsages: TapeUsage[],
    public sortOrder: number,
    public disabled: boolean,
  ) {}

  static fromAPIResponse({ id, name, price, notes, tape_usages, sort_order, disabled }): TapingMenuItem {
    return new TapingMenuItem(
      id ?? 0,
      name ?? "",
      price ?? 0,
      notes ?? "",
      tape_usages ?? [],
      sort_order ?? 0,
      disabled ?? false,
    );
  }

  // 日本語名の昇順コンパレータ。sortOrder（表示順）は比較に使わない
  // （マスタ側の「表示順」欄は廃止済みで、全画面が名前順で揃うことを優先する）。
  static sort(p: TapingMenuItem, n: TapingMenuItem): number {
    return p.name.localeCompare(n.name, "ja");
  }

  // 表示順のソートはここに集約する。ページ側で .sort() を書くと呼び出し漏れが起きるため、
  // API レスポンスを model に変換するこの choke point でソート済み配列を返す。
  static listFromAPIResponse(res: any[]): TapingMenuItem[] {
    return res.map(TapingMenuItem.fromAPIResponse).sort(TapingMenuItem.sort);
  }

  static draft(item?: TapingMenuItem): TapingMenuItemDraft {
    return {
      name: item?.name ?? "",
      price: item?.price ?? 0,
      notes: item?.notes ?? "",
      tape_usages: item?.tapeUsages ?? [],
      sort_order: item?.sortOrder ?? 0,
      disabled: item?.disabled ?? false,
    };
  }
}
