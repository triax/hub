import Member from "./Member";

export interface EquipDraft {
  name: string;
  for_practice: boolean;
  for_game: boolean;
  description: string;
}

export class Custody {
  constructor(
    public member_id: string,
    public timestamp: number,// t.Unix() * 1000
    public comment: string = "",
    public member: Member = null,
  ) { }
}

export default class Equip {
  constructor(
    public id: number,
    public name: string,
    public forPractice: boolean,
    public forGame: boolean,
    public description: string = "",
    public history: Custody[] = [],
  ) { }

  static fromAPIResponse({ id, key, name, for_practice, for_game, description, history }): Equip {
    return new Equip(id, name, for_practice, for_game, description, history ??= []);
  }
  static listFromAPIResponse(res: { id, key, name, for_practice, for_game, description, history }[]): Equip[] {
    return res.map(Equip.fromAPIResponse);
  }

  static draft(equip?: Equip): EquipDraft  {
    if (equip) {
      return { name: equip.name, for_practice: equip.forPractice, for_game: equip.forGame, description: equip.description };
    }
    return { name: "", for_practice: false, for_game: false, description: "" };
  }

  static sort(p: Equip, n: Equip): 1|-1 {
    return p.name < n.name ? -1 : 1;
  }
}
