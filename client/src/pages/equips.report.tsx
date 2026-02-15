import { useNavigate, useSearch } from "@tanstack/react-router";
import { useEffect, useMemo, useState } from "react";
import Layout, { LayoutProps } from "../../components/layout";
import Equip from "../../models/Equip";
import Member from "../../models/Member";
import EquipRepo, { CustodyRepo } from "../../repository/EquipRepo";
import MemberRepo from "../../repository/MemberRepo";
import { XIcon } from "@heroicons/react/outline";
import { useAppContext } from "../context";

export default function Report() {
  const { myself, startLoading, stopLoading } = useAppContext();
  const [equips, setEquips] = useState<Equip[]>([]);
  const [ids, setIDs] = useState<number[]>([]);
  const [principal, setPrincipal] = useState<Member>(null);
  const toggle = (y) => y ? (id) => setIDs(ids.filter(i => i != id)) : (id) => setIDs(ids.concat([id]));
  const repo = useMemo(() => new EquipRepo(), []);
  const navigate = useNavigate();
  const search: Record<string, string> = useSearch({ strict: false });
  if (equips.length) { stopLoading(); } else { startLoading(); }
  useEffect(() => {
    repo.list().then(setEquips);
  }, [repo]);
  return (
    <Layout>

      {(!search.proxy && myself?.slack?.profile?.title?.match(/staff/i)) ? <p
        className="text-right text-blue-500"
        onClick={() => navigate({ to: "/equips/report", search: { proxy: "1" } })}
      >代理入力する（Staff専用）</p> : null}

      <div className={search.proxy ? "border-b pb-4 border-black" : ""}>
        {search.proxy && !principal ? <ProxySelect setPrincipal={setPrincipal} /> : null}
        {principal ? <ProxyMemberCard member={principal} setPrincipal={setPrincipal} selected={true} /> : null}
        {principal ? <h2 className="text-xl font-bold">{getNames(principal)[0] + ' さんは、'}</h2> : null}
      </div>

      <h1 className="my-4 text-2xl font-bold">何を持って帰ってくれましたか?</h1>
      <div className="w-full">
        {equips.sort(Equip.sort).map(e => <EquipCard
          key={e.id} equip={e}
          selected={ids.includes(e.id)}
          toggle={toggle(ids.includes(e.id))}
        />)}
      </div>

      <div className="w-full text-center">
        <button
          onClick={() => {
            const li = equips.filter(e => ids.includes(e.id)).map(e => `・${e.name}`);
            if (!window.confirm(`以下のアイテムでよかったですか?\n${li.join("\n")}` + (principal ? `\n\n${getNames(principal)[0]}の【代理入力】` : ""))) return;
            (new CustodyRepo()).report(ids, (principal || myself), "").then(() => {
              window.alert("Thank you!!");
              navigate({ to: "/" });
            });
          }}
          disabled={ids.length == 0}
          className={`w-5/6 text-xl text-white p-4 rounded-md ` + (ids.length ? `bg-blue-600` : `bg-gray-200`)}
        >
          {ids.length ? `上記${ids.length}個の備品回収を記録する` : `選択してください`}
        </button>
      </div>
    </Layout>
  )
}

function EquipCard({ equip, selected, toggle }: { equip: Equip, selected: boolean, toggle: (number) => void }) {
  const card = selected ? "bg-red-800 text-white shadow-md" : "bg-gray-100"
  return (
    <div
      onClick={() => toggle(equip.id)}
      className={`flex rounded-lg px-4 py-2 mb-6 ${card}`}
    >
      <input
        onClick={() => toggle(equip.id)}
        className="mr-4 leading-tight w-6 h-6" type="checkbox"
        checked={selected}
        readOnly={true}
      />
      <span
        className="text-xl"
      >{equip.name}</span>
    </div>
  )
}

function ProxySelect(props: { setPrincipal: (Member) => void }) {
  const { setPrincipal } = props;
  const repo = useMemo(() => new MemberRepo(), []);
  const [candidates, setCandidates] = useState<Member[]>([]);
  const [keyword, setKeyword] = useState<string>("");
  return (
    <div>
      <h1 className="my-4 text-2xl font-bold">誰の代理ですか？</h1>
      <div className="w-full flex">
        <input type="text" className="border-gray-200 rounded-sm flex-1"
          onChange={ev => setKeyword(ev.target.value)}
        />
        <button
          className="bg-gray-200 px-8"
          onClick={async () => setCandidates(await repo.list({ keyword }))}
        >検索</button>
      </div>
      {candidates.length == 0 ? <p className="text-center p-8 text-gray-400">一致なし</p> : null}
      {candidates.map(candi => <ProxyMemberCard key={candi.slack.id} member={candi} setPrincipal={setPrincipal} />)}
    </div>
  )
}

function ProxyMemberCard(props: { member: Member, setPrincipal: (Member) => void, selected?: boolean }) {
  const { member: m, setPrincipal, selected } = props;
  const names = getNames(m);
  return (
    <div className={`p-2 rounded-md flex mt-4 ` + (selected ? "border-2" : "bg-gray-100")}
      onClick={() => selected ? null : setPrincipal(m)}
    >
      <div className="w-8 h-8 flex-none">
        <img
          src={m.slack.profile.image_512}
          alt={m.slack.profile.real_name}
          className="flex-none w-12 h-12 rounded-md object-cover bg-gray-100"
        />
      </div>
      <div className="flex-1 divide-x flex flex-wrap items-center">
        {names.map(name => <div key={name} className="px-2">{name}</div>)}
      </div>
      {selected ? <div className="flex items-center">
        <XIcon className="w-6 h-6"
          onClick={() => setPrincipal(null)}
        />
      </div> : null}
    </div>
  )
}

function getNames(m?: Member): string[] {
  if (!m) return [];
  return Array.from((new Set([m.slack.profile.real_name, m.slack.profile.display_name, m.slack.name, m.slack.real_name].filter(Boolean))).values());
}
