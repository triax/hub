import { PlayerNumber } from "../models/PlayerNumber";

export class PlayerNumberRepo {
    constructor(
        public baseURL = process.env.API_BASE_URL,
    ) { }
    all(): Promise<PlayerNumber[]> {
        return fetch(`${this.baseURL}/api/1/numbers`)
            .then((res) => res.json())
            .then((json) => json.map((p) => new PlayerNumber(p.number, p.uniforms, p.player_id)));
    }
    assign(pn: PlayerNumber, player_id: string, deprive: boolean): Promise<Response> { // TODO: ちゃんとする
        const endpoint = `${this.baseURL}/api/1/numbers/${pn.number}/${deprive ? "deprive" : "assign"}`;
        return fetch(endpoint, {method: "POST", body: JSON.stringify({ player_id })});
    }
}