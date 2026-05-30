export interface TapingMenuItemDraft {
  name: string;
  price: number;
  estimated_rolls: number;
  notes: string;
  sort_order: number;
  disabled: boolean;
}

export default class TapingMenuItem {
  constructor(
    public id: number,
    public name: string,
    public price: number,
    public estimatedRolls: number,
    public notes: string,
    public sortOrder: number,
    public disabled: boolean,
  ) {}

  static fromAPIResponse({ id, name, price, estimated_rolls, notes, sort_order, disabled }): TapingMenuItem {
    return new TapingMenuItem(
      id ?? 0,
      name ?? "",
      price ?? 0,
      estimated_rolls ?? 0,
      notes ?? "",
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
      estimated_rolls: item?.estimatedRolls ?? 0,
      notes: item?.notes ?? "",
      sort_order: item?.sortOrder ?? 0,
      disabled: item?.disabled ?? false,
    };
  }
}
