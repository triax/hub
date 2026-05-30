import { TapeUsage } from "./TapingMenuItem";

export default class Taping {
  constructor(
    public memberID: string,
    public eventID: string,
    public menuItemID: number,
    public menuItemName: string,
    public price: number,
    public tapeUsages: TapeUsage[],
    public requestedAt: number,
  ) {}

  static fromAPIResponse({ member_id, event_id, menu_item_id, menu_item_name, price, tape_usages, requested_at }): Taping {
    return new Taping(
      member_id ?? "",
      event_id ?? "",
      menu_item_id ?? 0,
      menu_item_name ?? "",
      price ?? 0,
      tape_usages ?? [],
      requested_at ?? 0,
    );
  }

  static listFromAPIResponse(res: any[]): Taping[] {
    return res.map(Taping.fromAPIResponse);
  }
}
