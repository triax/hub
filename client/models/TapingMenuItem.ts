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

  static listFromAPIResponse(res: any[]): TapingMenuItem[] {
    return res.map(TapingMenuItem.fromAPIResponse);
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
