import { useNavigate } from "@tanstack/react-router";
import { useEffect, useMemo, useState } from "react";
import Layout from "../../components/layout";
import Equip from "../../models/Equip";
import EquipRepo from "../../repository/EquipRepo";
import Member from "../../models/Member";
import { MemberCache } from "../../repository/MemberRepo";
import { useAppContext } from "../context";

export default function List() {
  const { myself, startLoading, stopLoading } = useAppContext();
  const [equips, setEquips] = useState<Equip[]>([]);
  const repo = useMemo(() => new EquipRepo(), []);
  const navigate = useNavigate();
  if (equips.length) { stopLoading(); } else { startLoading(); }
  useEffect(() => {
    repo.list().then(setEquips);
  }, [repo]);

  const takeHome = equips.filter(e => e.storageType !== "warehouse").sort(Equip.sort);
  const warehouse = equips.filter(e => e.storageType === "warehouse").sort(Equip.sort);

  return (
    <Layout>
      <EquipSection title="持ち帰り管理" equips={takeHome} navigate={navigate} />
      <EquipSection title="倉庫管理"     equips={warehouse} navigate={navigate} />
      <div className="
        px-4 sm:px-6 lg:px-8 py-4
        fixed left-0 bottom-0
        w-full flex flex-row-reverse
        space-x-4 space-x-reverse
      ">
        <div
          className="basis-1/3 text-center bg-blue-700 text-white p-2 rounded-md shadow-md shadow-gray-500"
          onClick={() => navigate({ to: "/equips/report" })}
        >回収報告</div>
        {myself?.slack?.profile?.title?.match(/staff/i) ? <div
          className="basis-2/3 text-center bg-red-900 text-white p-2 rounded-md shadow-md shadow-gray-500"
          onClick={() => navigate({ to: "/equips/create" })}
        >新規アイテム登録</div> : null}
      </div>
    </Layout>
  )
}

function EquipSection({ title, equips, navigate }: { title: string, equips: Equip[], navigate: any }) {
  if (equips.length === 0) return null;
  return (
    <div className="shadow overflow-hidden border border-gray-200 sm:rounded-lg mb-4">
      <div className="px-4 py-2 bg-gray-50 text-sm font-medium text-gray-500 border-b border-gray-200">{title}</div>
      <table className="min-w-full divide-y divide-gray-200">
        <tbody>
          {equips.map((eq, i) => <EquipItem
            key={eq.id} equip={eq} border={i < equips.length - 1}
            jump={() => navigate({ to: `/equips/${eq.id}` })}
          />)}
        </tbody>
      </table>
    </div>
  );
}

function Circle({ type }: { type: "practice" | "game" }) {
  switch (type) {
  case "game":
    return <div className="bg-orange-400 text-center w-4 h-4 rounded-full text-orange-200 text-xs">G</div>
  case "practice":
  default:
    return <div className="bg-teal-400 text-center w-4 h-4 rounded-full text-teal-200 text-xs">P</div>
  }
}

function EquipItem({ equip, jump, border }: { equip: Equip, jump: () => void, border: boolean }) {
  const [m, setMember] = useState<Member>(null)
  useEffect(() => {
    if (equip.history.length == 0) return;
    (new MemberCache()).get(equip.history[0].member_id).then(setMember);
  }, [equip]);
  return (
    <tr key={equip.id} onClick={jump} className={border ? "border-b" : ""}>
      <td className="pl-2">{m?.slack ? <div className="w-6 h-6 rounded-full overflow-hidden"><img
        src={m?.slack?.profile?.image_512}
        alt={m?.slack?.profile?.real_name}
        className="flex-none w-12 h-12 rounded-md object-cover"
      /></div> : null}</td>
      <td className="p-2">{equip.name}</td>
      <td className="p-2 w-8">{equip.forPractice ? <Circle type="practice" /> : null}</td>
      <td className="p-2 w-8">{equip.forGame ?     <Circle type="game" />     : null}</td>
    </tr>
  )
}
