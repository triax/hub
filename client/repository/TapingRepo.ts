import Taping from "../models/Taping";
import TapeItem from "../models/TapeItem";
import TapingMenuItem, { TapingMenuItemDraft } from "../models/TapingMenuItem";
import TeamEvent from "../models/TriaxEvent";
import { fetchJSON } from "./fetch";

export default class TapingRepo {
  constructor(
    public baseURL = import.meta.env.VITE_API_BASE_URL || "",
  ) {}

  // TapeItem 管理
  tapeItemList(): Promise<TapeItem[]> {
    return fetchJSON(this.baseURL + "/api/1/tape-items")
      .then(TapeItem.listFromAPIResponse);
  }

  tapeItemCreate(draft: { name: string; stock_count: number; sort_order: number; disabled: boolean }): Promise<TapeItem> {
    return fetchJSON(this.baseURL + "/api/1/tape-items", {
      method: "POST",
      body: JSON.stringify(draft),
    }).then(TapeItem.fromAPIResponse);
  }

  tapeItemUpdate(id: number, draft: { name: string; stock_count: number; sort_order: number; disabled: boolean }): Promise<TapeItem> {
    return fetchJSON(this.baseURL + `/api/1/tape-items/${id}/update`, {
      method: "POST",
      body: JSON.stringify(draft),
    }).then(TapeItem.fromAPIResponse);
  }

  tapeItemDelete(id: number): Promise<{ id: number }> {
    return fetchJSON(this.baseURL + `/api/1/tape-items/${id}/delete`, { method: "POST" });
  }

  // メニュー管理
  menuList(): Promise<TapingMenuItem[]> {
    return fetchJSON(this.baseURL + "/api/1/taping/menu")
      .then(TapingMenuItem.listFromAPIResponse);
  }

  menuCreate(draft: TapingMenuItemDraft): Promise<TapingMenuItem> {
    return fetchJSON(this.baseURL + "/api/1/taping/menu", {
      method: "POST",
      body: JSON.stringify(draft),
    }).then(TapingMenuItem.fromAPIResponse);
  }

  menuUpdate(id: number, draft: TapingMenuItemDraft): Promise<TapingMenuItem> {
    return fetchJSON(this.baseURL + `/api/1/taping/menu/${id}/update`, {
      method: "POST",
      body: JSON.stringify(draft),
    }).then(TapingMenuItem.fromAPIResponse);
  }

  menuDelete(id: number): Promise<{ id: number }> {
    return fetchJSON(this.baseURL + `/api/1/taping/menu/${id}/delete`, { method: "POST" });
  }

  // リクエスト
  submitRequest(eventID: string, menuItemIDs: number[]): Promise<Taping[]> {
    return fetchJSON(this.baseURL + "/api/1/taping/requests", {
      method: "POST",
      body: JSON.stringify({ event_id: eventID, menu_item_ids: menuItemIDs }),
    }).then(Taping.listFromAPIResponse);
  }

  getMyRequest(eventID: string): Promise<Taping[]> {
    return fetchJSON(this.baseURL + `/api/1/taping/requests/me?event_id=${encodeURIComponent(eventID)}`)
      .then(Taping.listFromAPIResponse);
  }

  listRequests(eventID?: string, year?: number): Promise<Taping[]> {
    const params = new URLSearchParams();
    if (eventID) params.set("event_id", eventID);
    if (year) params.set("year", String(year));
    const q = params.toString() ? `?${params}` : "";
    return fetchJSON(this.baseURL + `/api/1/taping/requests${q}`)
      .then(Taping.listFromAPIResponse);
  }

  // イベント一覧（直近40日）
  listEvents(): Promise<TeamEvent[]> {
    return fetchJSON(this.baseURL + "/api/1/taping/events")
      .then((res: any[]) => res.map(TeamEvent.fromAPIResponse).reverse()); // 新しい順
  }
}
