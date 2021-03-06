import { useRouter } from "next/router";
import { useState } from "react";
import Layout from "../../components/layout";
import Equip, { EquipDraft } from "../../models/Equip";
import EquipRepo from "../../repository/EquipRepo";

export default function CreateItem(props) {
  const [draft, setDraft] = useState<EquipDraft>(Equip.draft());
  const repo = new EquipRepo();
  const router = useRouter();
  return (
    <Layout {...props}>
      <h1 className="my-4 text-2xl font-bold">あたらしくアイテムを追加</h1>

      <div className="w-full">
        <form className="bg-white shadow-md rounded px-8 pt-6 pb-8 mb-4">
          <div className="mb-4">
            <label className="block text-gray-700 text-sm font-bold mb-2" htmlFor="name">
              備品名称
            </label>
            <input
              onChange={ev => setDraft({ ...draft, name: ev.target.value })}
              className="shadow appearance-none border rounded w-full py-2 px-3 text-gray-700 leading-tight focus:outline-none focus:shadow-outline"
              type="text" placeholder="例: 松葉杖"
            />
          </div>
          <div className="mb-6">
            <label className="md:w-2/3 block text-gray-500 font-bold">
              <input
                onChange={ev => setDraft({ ...draft, for_practice: ev.target.checked })}
                className="mr-2 leading-tight" type="checkbox"
              />
              <span className="">練習で必要</span>
            </label>
          </div>
          <div className="mb-6">
            <label className="md:w-2/3 block text-gray-500 font-bold">
              <input
                onChange={ev => setDraft({ ...draft, for_game: ev.target.checked })}
                className="mr-2 leading-tight" type="checkbox"
              />
              <span className="">試合で必要</span>
            </label>
          </div>



          <div className="mb-6">
            <label className="block text-gray-700 text-sm font-bold mb-2" htmlFor="description">
              詳細説明 (任意)
            </label>
            <textarea
              onChange={ev => setDraft({ ...draft, description: ev.target.value })}
              className="form-control block w-full
                px-3 py-1.5 m-0
                text-base
                font-norma
                text-gray-700 bg-white bg-clip-padding
                border border-solid border-gray-300
                rounded transition ease-in-out
              focus:text-gray-700 focus:bg-white focus:border-blue-600 focus:outline-none"
              id="description"
              rows={3}
              placeholder="内容物や、個数など、特筆事項があればここに記入"
            ></textarea>
          </div>

          <div className="flex items-center justify-between">
            <button
              onClick={() => repo.post(draft).then(() => router.push(`/equips`)).catch(err => console.log(err))}
              className="bg-blue-500 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded focus:outline-none focus:shadow-outline"
              type="button"
            >
              上記のとおり作成
            </button>
            <a className="inline-block align-baseline font-bold text-sm text-blue-500 hover:text-blue-800"
              onClick={() => router.push("/equips")}>
              キャンセル
            </a>
          </div>

        </form>
      </div>
    </Layout>
  )
}