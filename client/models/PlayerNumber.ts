import Member from "./Member";

export class PlayerNumber {
    public player?: Member; // Populated field
    constructor(
        public number: number,
        public uniforms: Uniform[],
        public player_id: string, // Slack ID
    ) { }

    static fromResponse(data: any) {
        return data.map((p) => new PlayerNumber(p.number, p.uniforms, p.player_id));
    }
}

export class Uniform {
    public owner?: Member; // Populated field
    constructor(
        public number: number,
        public size: string,
        public color: boolean,
        public damaged: boolean,
        public owner_id: string, // Slack ID
    ) { }
}