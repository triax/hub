import { PlayerNumber } from "../models/PlayerNumber";
import { fetchJSON } from "./fetch";

export class PlayerNumberRepo {
  constructor(
    public baseURL = import.meta.env.VITE_API_BASE_URL || "",
  ) { }
  all(): Promise<PlayerNumber[]> {
    return fetchJSON<PlayerNumber[]>(`${this.baseURL}/api/1/numbers`)
      .then((json) => json.map((p) => new PlayerNumber(p.number, p.uniforms, p.player_id)));
  }
  assign(pn: PlayerNumber, player_id: string, deprive: boolean): Promise<Response> { // TODO: ちゃんとする
    const endpoint = `${this.baseURL}/api/1/numbers/${pn.number}/${deprive ? "deprive" : "assign"}`;
    return fetchJSON(endpoint, {method: "POST", body: JSON.stringify({ player_id })});
  }
}
