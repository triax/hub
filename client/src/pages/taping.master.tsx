import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import Layout from "../../components/layout";
import TapeItem from "../../models/TapeItem";
import TapingMenuItem, { TapingMenuItemDraft, TapeUsage } from "../../models/TapingMenuItem";
import TapingRepo from "../../repository/TapingRepo";
import { useAppContext } from "../context";

function isTapingManager(myself: any): boolean {
  if (!myself?.slack?.id || myself.slack.id === "xxx") return false;
  if (myself.slack.is_admin) return true;
  return !!(myself.slack.profile?.title?.match(/trainer/i));
}

const emptyDraft = (): TapingMenuItemDraft => ({
  name: "", price: 0, notes: "", tape_usages: [], sort_order: 0, disabled: false,
});

export default function TapingMaster() {
  const { myself } = useAppContext();
  const navigate = useNavigate();
  const repo = useMemo(() => new TapingRepo(), []);
  const [items, setItems] = useState<TapingMenuItem[]>([]);
  const [tapeItems, setTapeItems] = useState<TapeItem[]>([]);
  const [editing, setEditing] = useState<TapingMenuItem | null>(null);
  const [draft, setDraft] = useState<TapingMenuItemDraft>(emptyDraft());
  const [showForm, setShowForm] = useState(false);
  const [activeTab, setActiveTab] = useState<"menu" | "tape">("menu");
  const [editingTape, setEditingTape] = useState<TapeItem | null>(null);
  const [tapeDraft, setTapeDraft] = useState({ name: "", sort_order: 0, disabled: false });
  const [showTapeForm, setShowTapeForm] = useState(false);

  useEffect(() => {
    if (myself?.slack?.id && myself.slack.id !== "xxx" && !isTapingManager(myself)) {
      navigate({ to: "/" });
    }
  }, [myself, navigate]);

  useEffect(() => {
    repo.menuList().then(setItems);
    repo.tapeItemList().then(setTapeItems);
  }, [repo]);

  // --- メニュー操作 ---
  const openCreate = () => { setEditing(null); setDraft(emptyDraft()); setShowForm(true); };
  const openEdit = (item: TapingMenuItem) => { setEditing(item); setDraft(TapingMenuItem.draft(item)); setShowForm(true); };
  const saveMenu = async () => {
    if (editing) { await repo.menuUpdate(editing.id, draft); }
    else { await repo.menuCreate(draft); }
    setShowForm(false);
    repo.menuList().then(setItems);
  };
  const removeMenu = async (item: TapingMenuItem) => {
    if (!confirm(`「${item.name}」を削除しますか？`)) return;
    await repo.menuDelete(item.id);
    repo.menuList().then(setItems);
  };

  const setUsageQty = (tapeItemID: number, tapeItemName: string, qty: number) => {
    const usages = draft.tape_usages.filter(u => u.tape_item_id !== tapeItemID);
    if (qty > 0) usages.push({ tape_item_id: tapeItemID, tape_item_name: tapeItemName, quantity: qty });
    setDraft({ ...draft, tape_usages: usages });
  };
  const getUsageQty = (tapeItemID: number) =>
    draft.tape_usages.find(u => u.tape_item_id === tapeItemID)?.quantity ?? 0;

  // --- テープ素材操作 ---
  const openTapeCreate = () => { setEditingTape(null); setTapeDraft({ name: "", sort_order: 0, disabled: false }); setShowTapeForm(true); };
  const openTapeEdit = (t: TapeItem) => { setEditingTape(t); setTapeDraft({ name: t.name, sort_order: t.sortOrder, disabled: t.disabled }); setShowTapeForm(true); };
  const saveTape = async () => {
    if (editingTape) { await repo.tapeItemUpdate(editingTape.id, tapeDraft); }
    else { await repo.tapeItemCreate(tapeDraft); }
    setShowTapeForm(false);
    repo.tapeItemList().then(setTapeItems);
  };
  const removeTape = async (t: TapeItem) => {
    if (!confirm(`「${t.name}」を削除しますか？`)) return;
    await repo.tapeItemDelete(t.id);
    repo.tapeItemList().then(setTapeItems);
  };

  if (myself?.slack?.id && myself.slack.id !== "xxx" && !isTapingManager(myself)) return null;

  return (
    <Layout>
      <div className="px-4 py-6 max-w-2xl mx-auto">
        <h1 className="text-xl font-bold mb-4">テーピング管理</h1>

        {/* タブ */}
        <div className="flex border-b mb-4">
          {(["menu", "tape"] as const).map(tab => (
            <button
              key={tab}
              className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px ${activeTab === tab ? "border-blue-700 text-blue-700" : "border-transparent text-gray-500"}`}
              onClick={() => setActiveTab(tab)}
            >{tab === "menu" ? "施術メニュー" : "テープ素材"}</button>
          ))}
        </div>

        {/* 施術メニュータブ */}
        {activeTab === "menu" && (
          <>
            <div className="flex justify-end mb-3">
              <button className="bg-blue-700 text-white px-4 py-2 rounded-md text-sm" onClick={openCreate}>+ 追加</button>
            </div>
            <div className="shadow overflow-hidden border border-gray-200 rounded-lg">
              <table className="min-w-full divide-y divide-gray-200 text-sm">
                <thead className="bg-gray-50">
                  <tr>
                    <th className="px-3 py-2 text-left text-xs font-medium text-gray-500 uppercase">名称</th>
                    <th className="px-3 py-2 text-right text-xs font-medium text-gray-500 uppercase">単価</th>
                    <th className="px-3 py-2 text-left text-xs font-medium text-gray-500 uppercase">テープ</th>
                    <th className="px-3 py-2 text-left text-xs font-medium text-gray-500 uppercase">備考</th>
                    <th className="px-3 py-2"></th>
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-200">
                  {items.map(item => (
                    <tr key={item.id} className={item.disabled ? "opacity-40" : ""}>
                      <td className="px-3 py-2 font-medium">{item.name}</td>
                      <td className="px-3 py-2 text-right">¥{item.price}</td>
                      <td className="px-3 py-2 text-gray-500 text-xs">
                        {item.tapeUsages?.map(u => `${u.tape_item_name}×${u.quantity}`).join(" / ") || "—"}
                      </td>
                      <td className="px-3 py-2 text-gray-500">{item.notes}</td>
                      <td className="px-3 py-2 text-right space-x-2">
                        <button className="text-blue-600 hover:underline" onClick={() => openEdit(item)}>編集</button>
                        <button className="text-red-600 hover:underline" onClick={() => removeMenu(item)}>削除</button>
                      </td>
                    </tr>
                  ))}
                  {items.length === 0 && (
                    <tr><td colSpan={5} className="px-3 py-4 text-center text-gray-400">メニューがありません</td></tr>
                  )}
                </tbody>
              </table>
            </div>
          </>
        )}

        {/* テープ素材タブ */}
        {activeTab === "tape" && (
          <>
            <div className="flex justify-end mb-3">
              <button className="bg-blue-700 text-white px-4 py-2 rounded-md text-sm" onClick={openTapeCreate}>+ 追加</button>
            </div>
            <div className="shadow overflow-hidden border border-gray-200 rounded-lg">
              <table className="min-w-full divide-y divide-gray-200 text-sm">
                <thead className="bg-gray-50">
                  <tr>
                    <th className="px-3 py-2 text-left text-xs font-medium text-gray-500 uppercase">名称</th>
                    <th className="px-3 py-2 text-center text-xs font-medium text-gray-500 uppercase">状態</th>
                    <th className="px-3 py-2"></th>
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-200">
                  {tapeItems.map(t => (
                    <tr key={t.id} className={t.disabled ? "opacity-40" : ""}>
                      <td className="px-3 py-2 font-medium">{t.name}</td>
                      <td className="px-3 py-2 text-center text-xs">
                        {t.disabled ? <span className="text-gray-400">無効</span> : <span className="text-green-600">有効</span>}
                      </td>
                      <td className="px-3 py-2 text-right space-x-2">
                        <button className="text-blue-600 hover:underline" onClick={() => openTapeEdit(t)}>編集</button>
                        <button className="text-red-600 hover:underline" onClick={() => removeTape(t)}>削除</button>
                      </td>
                    </tr>
                  ))}
                  {tapeItems.length === 0 && (
                    <tr><td colSpan={3} className="px-3 py-4 text-center text-gray-400">テープ素材がありません</td></tr>
                  )}
                </tbody>
              </table>
            </div>
          </>
        )}

        {/* 施術メニュー フォーム */}
        {showForm && (
          <div className="fixed inset-0 bg-black bg-opacity-40 flex items-center justify-center z-50">
            <div className="bg-white rounded-lg shadow-xl p-6 w-full max-w-md mx-4 max-h-[90vh] overflow-y-auto">
              <h2 className="text-lg font-bold mb-4">{editing ? "メニュー編集" : "メニュー追加"}</h2>
              <div className="space-y-3">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">名称 <span className="text-red-500">*</span></label>
                  <input type="text" className="w-full border border-gray-300 rounded-md p-2 text-sm"
                    value={draft.name} onChange={e => setDraft({ ...draft, name: e.target.value })} />
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">単価目安（円）</label>
                    <input type="number" className="w-full border border-gray-300 rounded-md p-2 text-sm"
                      value={draft.price} onChange={e => setDraft({ ...draft, price: Number(e.target.value) })} />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">表示順</label>
                    <input type="number" className="w-full border border-gray-300 rounded-md p-2 text-sm"
                      value={draft.sort_order} onChange={e => setDraft({ ...draft, sort_order: Number(e.target.value) })} />
                  </div>
                </div>
                {/* テープ使用量 */}
                {tapeItems.filter(t => !t.disabled).length > 0 && (
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">テープ使用量（本）</label>
                    <div className="space-y-2">
                      {tapeItems.filter(t => !t.disabled).map(t => (
                        <div key={t.id} className="flex items-center space-x-2">
                          <span className="text-sm flex-1">{t.name}</span>
                          <input
                            type="number" step="0.5" min="0"
                            className="w-20 border border-gray-300 rounded-md p-1 text-sm text-right"
                            value={getUsageQty(t.id)}
                            onChange={e => setUsageQty(t.id, t.name, Number(e.target.value))}
                          />
                        </div>
                      ))}
                    </div>
                  </div>
                )}
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">備考</label>
                  <input type="text" className="w-full border border-gray-300 rounded-md p-2 text-sm"
                    value={draft.notes} onChange={e => setDraft({ ...draft, notes: e.target.value })} />
                </div>
                <label className="flex items-center space-x-2 cursor-pointer">
                  <input type="checkbox" checked={draft.disabled}
                    onChange={e => setDraft({ ...draft, disabled: e.target.checked })} />
                  <span className="text-sm text-gray-700">無効にする</span>
                </label>
              </div>
              <div className="flex justify-end space-x-3 mt-5">
                <button className="px-4 py-2 text-sm border border-gray-300 rounded-md" onClick={() => setShowForm(false)}>キャンセル</button>
                <button className="px-4 py-2 text-sm bg-blue-700 text-white rounded-md disabled:opacity-50"
                  onClick={saveMenu} disabled={!draft.name}>保存</button>
              </div>
            </div>
          </div>
        )}

        {/* テープ素材 フォーム */}
        {showTapeForm && (
          <div className="fixed inset-0 bg-black bg-opacity-40 flex items-center justify-center z-50">
            <div className="bg-white rounded-lg shadow-xl p-6 w-full max-w-sm mx-4">
              <h2 className="text-lg font-bold mb-4">{editingTape ? "テープ素材編集" : "テープ素材追加"}</h2>
              <div className="space-y-3">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">名称 <span className="text-red-500">*</span></label>
                  <input type="text" className="w-full border border-gray-300 rounded-md p-2 text-sm"
                    value={tapeDraft.name} onChange={e => setTapeDraft({ ...tapeDraft, name: e.target.value })} />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">表示順</label>
                  <input type="number" className="w-full border border-gray-300 rounded-md p-2 text-sm"
                    value={tapeDraft.sort_order} onChange={e => setTapeDraft({ ...tapeDraft, sort_order: Number(e.target.value) })} />
                </div>
                <label className="flex items-center space-x-2 cursor-pointer">
                  <input type="checkbox" checked={tapeDraft.disabled}
                    onChange={e => setTapeDraft({ ...tapeDraft, disabled: e.target.checked })} />
                  <span className="text-sm text-gray-700">無効にする</span>
                </label>
              </div>
              <div className="flex justify-end space-x-3 mt-5">
                <button className="px-4 py-2 text-sm border border-gray-300 rounded-md" onClick={() => setShowTapeForm(false)}>キャンセル</button>
                <button className="px-4 py-2 text-sm bg-blue-700 text-white rounded-md disabled:opacity-50"
                  onClick={saveTape} disabled={!tapeDraft.name}>保存</button>
              </div>
            </div>
          </div>
        )}
      </div>
    </Layout>
  );
}
