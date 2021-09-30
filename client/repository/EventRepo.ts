// import Member from "../models/Member";

import TeamEvent from "../models/TriaxEvent";

export default class TeamEventRepo {
  constructor(
    public baseURL = process.env.API_BASE_URL,
  ) { }
  get(id: string): Promise<TeamEvent> {
    const endpoint = this.baseURL + `/api/1/events/${id}`;
    return fetch(endpoint).then(res => res.json()).then(TeamEvent.fromAPIResponse);
  }
  delete(id: string): Promise<{id: string, ok: boolean}> {
    const endpoint = this.baseURL + `/api/1/events/${id}/delete`;
    return fetch(endpoint, { method: "POST" }).then(res => res.json());
  }
  list(): Promise<TeamEvent[]> {
    const endpoint = this.baseURL + `/api/1/events`;
    const res = fetch(endpoint);
    console.log(res);
    return res.then(res => res.json()).then(a => a.map(TeamEvent.fromAPIResponse));
  }
}
