import Equip, { EquipDraft } from "../models/Equip";
import Member from "../models/Member";

export default class EquipRepo {
  constructor(
    public baseURL = process.env.API_BASE_URL,
  ) { }
  list(): Promise<Equip[]> {
    const endpoint = this.baseURL + `/api/1/equips`;
    return fetch(endpoint).then(res => res.json()).then(Equip.listFromAPIResponse);
  }
  get(id: string): Promise<Equip> {
    const endpoint = this.baseURL + `/api/1/equips/${id}`;
    return fetch(endpoint).then(res => res.json()).then(Equip.fromAPIResponse);
  }
  post(draft: EquipDraft): Promise<Equip> {
    const endpoint = this.baseURL + `/api/1/equips`;
    return fetch(endpoint, {
      method: "POST",
      body: JSON.stringify(draft),
    }).then(res => res.json()).then(Equip.fromAPIResponse);
  }
  delete(id: number): any {
    const endpoint = this.baseURL + `/api/1/equips/${id}/delete`;
    return fetch(endpoint, {method: "POST"}).then(res => res.json());
  }
  update(id: number|string, draft: EquipDraft): Promise<any> {
    const endpoint = this.baseURL + `/api/1/equips/${id}/update`;
    return fetch(endpoint, { method: "POST", body: JSON.stringify(draft) }).then(res => res.json());
  }
}

export class CustodyRepo {
  constructor(
    public baseURL = process.env.API_BASE_URL,
  ) { }
  report(equipIDs: number[], reporter: Member, comment: string): any {
    const endpoint = this.baseURL + `/api/1/equips/custody`;
    return fetch(endpoint, {
      method: "POST",
      body: JSON.stringify({ ids: equipIDs, member_id: reporter.slack.id, comment }),
    }).then(res => res.json());
  }

}