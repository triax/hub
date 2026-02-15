// import Member from "../models/Member";

import TeamEvent from "../models/TriaxEvent";

export default class TeamEventRepo {
  constructor(
    public baseURL = import.meta.env.VITE_API_BASE_URL || "",
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
    return fetch(this.baseURL + "/api/1/events")
      .then(res => res.json())
      .then(res => res.map(TeamEvent.fromAPIResponse));
  }
  async rsvp({event, answer, params}): Promise<TeamEvent> {
    const endpoint = this.baseURL + "/api/1/events/answer";
    return fetch(endpoint, { method: "POST", body: JSON.stringify({
      event: { id: event.google.id }, type: answer, params: params ? params : null,
    })})
      .then(res => res.json())
      .then(TeamEvent.fromAPIResponse);
  }
}