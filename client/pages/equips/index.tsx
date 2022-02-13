import { Router, useRouter } from "next/router";
import { useEffect, useMemo, useState } from "react";
import Layout from "../../components/layout";
import Equip from "../../models/Equip";
import EquipRepo from "../../repository/EquipRepo";
import { PlusIcon } from "@heroicons/react/outline";

export default function List(props) {
  const [equips, setEquips] = useState<Equip[]>([]);
  const repo = useMemo(() => new EquipRepo(), []);
  const router = useRouter();
  useEffect(() => {
    repo.list().then(setEquips);
  }, [repo]);
  return (
    <Layout {...props}>
      <div className="shadow overflow-hidden border-b border-gray-200 sm:rounded-lg">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <th className="col text-left p-2">名称</th>
            <th className="col text-left p-2">練習</th>
            <th className="col text-left p-2">試合</th>
          </thead>
          <tbody>
            {equips.map(eq => <EquipItem key={eq.id} equip={eq} jump={() => router.push(`/equips/${eq.id}`)}/>)}
          </tbody>
        </table>
      </div>
      {props.myself?.slack?.is_admin ?
        <div className="py-8">
          <div
            className="w-full p-4 rounded-md bg-teal-400 text-teal-800 text-center cursor-pointer flex justify-center"
            onClick={() => router.push("/equips/create")}
          ><PlusIcon className="w-4 mr-2" aria-hidden={true} /><span>新規登録</span></div>
        </div> : null}
    </Layout>
  )
}

function EquipItem({ equip, jump }: { equip: Equip, jump }) {
  return (
    <tr key={equip.id} onClick={jump}>
      <td className="p-2">{equip.name}</td>
      <td className="p-2">{equip.forPractice ? "✔" : ""}</td>
      <td className="p-2">{equip.forGame ? "✔" : ""}</td>
    </tr>
  )
}