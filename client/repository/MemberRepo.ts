import Member from "../models/Member";

export default class MemberRepo {
  constructor(
      public baseURL = process.env.API_BASE_URL,
  ) {}
  myself(): Promise<Member> {
    const endpoint = this.baseURL + `/api/1/myself`;
    return fetch(endpoint).then(res => res.json()).then(Member.fromAPIResponse);
  }
  get(id: string): Promise<Member> {
    const endpoint = this.baseURL + `/api/1/members/${id}`;
    return fetch(endpoint).then(res => res.json()).then(Member.fromAPIResponse);
  }
  updateStatus(id: string, status: string): Promise<Member> {
    const endpoint = this.baseURL + `/api/1/members/${id}/status`;
    return fetch(endpoint, {
      method: "POST",
      body: JSON.stringify({ status }),
    }).then(res => res.json()).then(Member.fromAPIResponse);
  }
}