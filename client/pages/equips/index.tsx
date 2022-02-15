import { Router, useRouter } from "next/router";
import { useEffect, useMemo, useState } from "react";
import Layout from "../../components/layout";
import Equip from "../../models/Equip";
import EquipRepo from "../../repository/EquipRepo";
import { PlusIcon } from "@heroicons/react/outline";
import Member from "../../models/Member";
import { MemberCache } from "../../repository/MemberRepo";
import Image from "next/image";

export default function List(props) {
  const [equips, setEquips] = useState<Equip[]>([]);
  const repo = useMemo(() => new EquipRepo(), []);
  const router = useRouter();
  useEffect(() => {
    repo.list().then(setEquips);
  }, [repo]);
  return (
    <Layout {...props}>
      {props.myself?.slack?.is_admin ? <span
        className="py-2 flex text-red-900"
        onClick={() => router.push("/equips/create")}
      >
        <PlusIcon className="w-4 mr-2" aria-hidden={true} />
        <span>新規アイテム登録</span>
      </span> : null}
      <div className="shadow overflow-hidden border border-gray-200 sm:rounded-lg">
        <table className="min-w-full divide-y divide-gray-200">
          <tbody>
            {equips.map((eq, i) => <EquipItem
              key={eq.id} equip={eq} border={i < equips.length - 1}
              jump={() => router.push(`/equips/${eq.id}`)}
            />)}
          </tbody>
        </table>
      </div>
      <div className="flex flex-row-reverse">
        <div
          className="w-1/3 text-center bg-blue-700 text-white p-2 my-2 rounded-md shadow"
          onClick={() => router.push("/equips/report")}
        >回収報告</div>
      </div>
    </Layout>
  )
}

function Circle({ type }: { type: "practice" | "game" }) {
  switch (type) {
  case "game":
    return <div className="bg-orange-400 text-center rounded-full text-orange-200 text-xs">G</div>
  case "practice":
  default:
    return <div className="bg-teal-400 text-center rounded-full text-teal-200 text-xs">P</div>
  }
}

function EquipItem({ equip, jump, border }: { equip: Equip, jump, border: boolean }) {
  const [m, setMember] = useState<Member>(null)
  useEffect(() => {
    if (equip.history.length == 0) return;
    (new MemberCache()).get(equip.history[0].member_id).then(setMember);
  }, []);
  return (
    <tr key={equip.id} onClick={jump} className={border ? "border-b" : ""}>
      <td className="pl-2">{m?.slack ? <div className="w-6 h-6 rounded-full overflow-hidden"><Image
        loader={({ src }) => src}
        unoptimized={true}
        src={m?.slack?.profile?.image_512}
        alt={m?.slack?.profile?.real_name}
        className="flex-none w-12 h-12 rounded-md object-cover bg-gray-100"
        width={120}
        height={120}
      /></div> : null}</td>
      <td className="p-2">{equip.name}</td>
      <td className="p-2">{equip.forPractice ? <Circle type="practice" /> : null}</td>
      <td className="p-2">{equip.forGame ?     <Circle type="game" />     : null}</td>
    </tr>
  )
}