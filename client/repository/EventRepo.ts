import TeamEvent from "../models/TriaxEvent";
import { fetchJSON } from "./fetch";

export default class TeamEventRepo {
  constructor(
    public baseURL = import.meta.env.VITE_API_BASE_URL || "",
  ) { }
  get(id: string): Promise<TeamEvent> {
    const endpoint = this.baseURL + `/api/1/events/${id}`;
    return fetchJSON(endpoint).then(TeamEvent.fromAPIResponse);
  }
  delete(id: string): Promise<{id: string, ok: boolean}> {
    const endpoint = this.baseURL + `/api/1/events/${id}/delete`;
    return fetchJSON(endpoint, { method: "POST" });
  }
  list(): Promise<TeamEvent[]> {
    return fetchJSON<TeamEvent[]>(this.baseURL + "/api/1/events")
      .then(res => res.map(TeamEvent.fromAPIResponse));
  }
  async rsvp({event, answer, params}): Promise<TeamEvent> {
    const endpoint = this.baseURL + "/api/1/events/answer";
    return fetchJSON(endpoint, { method: "POST", body: JSON.stringify({
      event: { id: event.google.id }, type: answer, params: params ? params : null,
    })})
      .then(TeamEvent.fromAPIResponse);
  }
}
