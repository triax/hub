import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import Layout from "../../components/layout";
import TapingMenuItem, { TapingMenuItemDraft } from "../../models/TapingMenuItem";
import TapingRepo from "../../repository/TapingRepo";
import { useAppContext } from "../context";

function isTapingManager(myself: any): boolean {
  if (!myself?.slack) return false;
  if (myself.slack.is_admin) return true;
  return !!(myself.slack.profile?.title?.match(/trainer/i));
}

const emptyDraft = (): TapingMenuItemDraft => ({
  name: "", price: 0, estimated_rolls: 0, notes: "", sort_order: 0, disabled: false,
});

export default function TapingMaster() {
  const { myself } = useAppContext();
  const navigate = useNavigate();
  const repo = useMemo(() => new TapingRepo(), []);
  const [items, setItems] = useState<TapingMenuItem[]>([]);
  const [editing, setEditing] = useState<TapingMenuItem | null>(null);
  const [draft, setDraft] = useState<TapingMenuItemDraft>(emptyDraft());
  const [showForm, setShowForm] = useState(false);

  // 権限チェック: placeholder(id="xxx") のうちは待つ
  useEffect(() => {
    if (!myself?.slack?.id || myself.slack.id === "xxx") return;
    if (!isTapingManager(myself)) {
      navigate({ to: "/" });
    }
  }, [myself, navigate]);

  useEffect(() => {
    repo.menuList().then(setItems);
  }, [repo]);

  const openCreate = () => {
    setEditing(null);
    setDraft(emptyDraft());
    setShowForm(true);
  };

  const openEdit = (item: TapingMenuItem) => {
    setEditing(item);
    setDraft(TapingMenuItem.draft(item));
    setShowForm(true);
  };

  const save = async () => {
    if (editing) {
      await repo.menuUpdate(editing.id, draft);
    } else {
      await repo.menuCreate(draft);
    }
    setShowForm(false);
    repo.menuList().then(setItems);
  };

  const remove = async (item: TapingMenuItem) => {
    if (!confirm(`「${item.name}」を削除しますか？`)) return;
    await repo.menuDelete(item.id);
    repo.menuList().then(setItems);
  };

  if (myself?.slack && !isTapingManager(myself)) return null;

  return (
    <Layout>
      <div className="px-4 py-6 max-w-2xl mx-auto">
        <div className="flex items-center justify-between mb-4">
          <h1 className="text-xl font-bold">テーピングメニュー管理</h1>
          <button
            className="bg-blue-700 text-white px-4 py-2 rounded-md text-sm"
            onClick={openCreate}
          >+ 追加</button>
        </div>

        {/* メニュー一覧 */}
        <div className="shadow overflow-hidden border border-gray-200 rounded-lg">
          <table className="min-w-full divide-y divide-gray-200 text-sm">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-3 py-2 text-left text-xs font-medium text-gray-500 uppercase">名称</th>
                <th className="px-3 py-2 text-right text-xs font-medium text-gray-500 uppercase">単価</th>
                <th className="px-3 py-2 text-right text-xs font-medium text-gray-500 uppercase">テープ目安</th>
                <th className="px-3 py-2 text-left text-xs font-medium text-gray-500 uppercase">備考</th>
                <th className="px-3 py-2 text-center text-xs font-medium text-gray-500 uppercase">状態</th>
                <th className="px-3 py-2"></th>
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-gray-200">
              {items.map(item => (
                <tr key={item.id} className={item.disabled ? "opacity-40" : ""}>
                  <td className="px-3 py-2 font-medium">{item.name}</td>
                  <td className="px-3 py-2 text-right">¥{item.price}</td>
                  <td className="px-3 py-2 text-right">{item.estimatedRolls}本</td>
                  <td className="px-3 py-2 text-gray-500">{item.notes}</td>
                  <td className="px-3 py-2 text-center">
                    {item.disabled ? <span className="text-gray-400">無効</span> : <span className="text-green-600">有効</span>}
                  </td>
                  <td className="px-3 py-2 text-right space-x-2">
                    <button className="text-blue-600 hover:underline" onClick={() => openEdit(item)}>編集</button>
                    <button className="text-red-600 hover:underline" onClick={() => remove(item)}>削除</button>
                  </td>
                </tr>
              ))}
              {items.length === 0 && (
                <tr><td colSpan={6} className="px-3 py-4 text-center text-gray-400">メニューがありません</td></tr>
              )}
            </tbody>
          </table>
        </div>

        {/* 作成・編集フォーム */}
        {showForm && (
          <div className="fixed inset-0 bg-black bg-opacity-40 flex items-center justify-center z-50">
            <div className="bg-white rounded-lg shadow-xl p-6 w-full max-w-md mx-4">
              <h2 className="text-lg font-bold mb-4">{editing ? "メニュー編集" : "メニュー追加"}</h2>
              <div className="space-y-3">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">名称 <span className="text-red-500">*</span></label>
                  <input
                    type="text"
                    className="w-full border border-gray-300 rounded-md p-2 text-sm"
                    value={draft.name}
                    onChange={e => setDraft({ ...draft, name: e.target.value })}
                  />
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">単価目安（円）</label>
                    <input
                      type="number"
                      className="w-full border border-gray-300 rounded-md p-2 text-sm"
                      value={draft.price}
                      onChange={e => setDraft({ ...draft, price: Number(e.target.value) })}
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">テープ目安（本）</label>
                    <input
                      type="number"
                      step="0.5"
                      className="w-full border border-gray-300 rounded-md p-2 text-sm"
                      value={draft.estimated_rolls}
                      onChange={e => setDraft({ ...draft, estimated_rolls: Number(e.target.value) })}
                    />
                  </div>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">備考</label>
                  <input
                    type="text"
                    className="w-full border border-gray-300 rounded-md p-2 text-sm"
                    value={draft.notes}
                    onChange={e => setDraft({ ...draft, notes: e.target.value })}
                  />
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">表示順</label>
                    <input
                      type="number"
                      className="w-full border border-gray-300 rounded-md p-2 text-sm"
                      value={draft.sort_order}
                      onChange={e => setDraft({ ...draft, sort_order: Number(e.target.value) })}
                    />
                  </div>
                  <div className="flex items-end pb-2">
                    <label className="flex items-center space-x-2 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={draft.disabled}
                        onChange={e => setDraft({ ...draft, disabled: e.target.checked })}
                      />
                      <span className="text-sm text-gray-700">無効にする</span>
                    </label>
                  </div>
                </div>
              </div>
              <div className="flex justify-end space-x-3 mt-5">
                <button
                  className="px-4 py-2 text-sm border border-gray-300 rounded-md"
                  onClick={() => setShowForm(false)}
                >キャンセル</button>
                <button
                  className="px-4 py-2 text-sm bg-blue-700 text-white rounded-md disabled:opacity-50"
                  onClick={save}
                  disabled={!draft.name}
                >保存</button>
              </div>
            </div>
          </div>
        )}
      </div>
    </Layout>
  );
}
