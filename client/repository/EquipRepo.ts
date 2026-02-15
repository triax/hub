import Equip, { EquipDraft } from "../models/Equip";
import Member from "../models/Member";
import { fetchJSON } from "./fetch";

export default class EquipRepo {
  constructor(
    public baseURL = import.meta.env.VITE_API_BASE_URL || "",
  ) { }
  list(): Promise<Equip[]> {
    const endpoint = this.baseURL + `/api/1/equips`;
    return fetchJSON(endpoint).then(Equip.listFromAPIResponse);
  }
  get(id: string): Promise<Equip> {
    const endpoint = this.baseURL + `/api/1/equips/${id}`;
    return fetchJSON(endpoint).then(Equip.fromAPIResponse);
  }
  post(draft: EquipDraft): Promise<Equip> {
    const endpoint = this.baseURL + `/api/1/equips`;
    return fetchJSON(endpoint, {
      method: "POST",
      body: JSON.stringify(draft),
    }).then(Equip.fromAPIResponse);
  }
  delete(id: number): any {
    const endpoint = this.baseURL + `/api/1/equips/${id}/delete`;
    return fetchJSON(endpoint, {method: "POST"});
  }
  update(id: number|string, draft: EquipDraft): Promise<any> {
    const endpoint = this.baseURL + `/api/1/equips/${id}/update`;
    return fetchJSON(endpoint, { method: "POST", body: JSON.stringify(draft) });
  }
}

export class CustodyRepo {
  constructor(
    public baseURL = import.meta.env.VITE_API_BASE_URL || "",
  ) { }
  report(equipIDs: number[], reporter: Member, comment: string): any {
    const endpoint = this.baseURL + `/api/1/equips/custody`;
    return fetchJSON(endpoint, {
      method: "POST",
      body: JSON.stringify({ ids: equipIDs, member_id: reporter.slack.id, comment }),
    });
  }

}
