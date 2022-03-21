import { useRouter } from "next/router";
import { useEffect, useMemo, useState } from "react";
import Layout from "../../components/layout";
import Equip from "../../models/Equip";
import EquipRepo from "../../repository/EquipRepo";
import Member from "../../models/Member";
import { MemberCache } from "../../repository/MemberRepo";
import Image from "next/image";

export default function List(props) {
  const { startLoading, stopLoading } = props;
  const [equips, setEquips] = useState<Equip[]>([]);
  const repo = useMemo(() => new EquipRepo(), []);
  const router = useRouter();
  equips.length ? stopLoading() : startLoading();
  useEffect(() => {
    repo.list().then(setEquips);
  }, [repo]);
  return (
    <Layout {...props}>
      <div className="shadow overflow-hidden border border-gray-200 sm:rounded-lg mb-8">
        <table className="min-w-full divide-y divide-gray-200">
          <tbody>
            {equips.sort(Equip.sort).map((eq, i) => <EquipItem
              key={eq.id} equip={eq} border={i < equips.length - 1}
              jump={() => router.push(`/equips/${eq.id}`)}
            />)}
          </tbody>
        </table>
      </div>
      {props.myself?.slack?.is_admin ? <span
        className="py-2 flex text-red-900"
        onClick={() => router.push("/equips/create")}
      >
        <span>+ 新規アイテム登録</span>
      </span> : null}
      <div
        className="
        px-4 sm:px-6 lg:px-8
        py-4
        fixed left-0 bottom-0
        w-full flex flex-row-reverse
        "
      >
        <div
          className="
            w-1/3 text-center bg-blue-700 text-white p-2 my-2 rounded-md
            shadow-md shadow-gray-500
          "
          onClick={() => router.push("/equips/report")}
        >回収報告</div>
      </div>
    </Layout>
  )
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

function EquipItem({ equip, jump, border }: { equip: Equip, jump, border: boolean }) {
  const [m, setMember] = useState<Member>(null)
  useEffect(() => {
    if (equip.history.length == 0) return;
    (new MemberCache()).get(equip.history[0].member_id).then(setMember);
  }, [equip]);
  return (
    <tr key={equip.id} onClick={jump} className={border ? "border-b" : ""}>
      <td className="pl-2">{m?.slack ? <div className="w-6 h-6 rounded-full overflow-hidden"><Image
        loader={({ src }) => src}
        unoptimized={true}
        src={m?.slack?.profile?.image_512}
        alt={m?.slack?.profile?.real_name}
        className="flex-none w-12 h-12 rounded-md object-cover"
        width={120}
        height={120}
      /></div> : null}</td>
      <td className="p-2">{equip.name}</td>
      <td className="p-2 w-8">{equip.forPractice ? <Circle type="practice" /> : null}</td>
      <td className="p-2 w-8">{equip.forGame ?     <Circle type="game" />     : null}</td>
    </tr>
  )
}