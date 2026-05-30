import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { ChevronRightIcon } from "@heroicons/react/outline";
import Layout from "../../components/layout";
import TapeItem from "../../models/TapeItem";
import TapingMenuItem, { TapingMenuItemDraft } from "../../models/TapingMenuItem";
import TapingRepo from "../../repository/TapingRepo";
import { useAppContext } from "../context";

function isTapingManager(myself: any): boolean {
  if (!myself?.slack?.id || myself.slack.id === "xxx") return false;
  if (myself.slack.is_admin) return true;
  return !!(myself.slack.profile?.title?.match(/trainer|staff/i));
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
  const [tapeDraft, setTapeDraft] = useState({ name: "", stock_count: 0, sort_order: 0, disabled: false });
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

  const setUsageQty = (tapeItemID: number, tapeItemName: string, qty: number) => {
    const usages = draft.tape_usages.filter(u => u.tape_item_id !== tapeItemID);
    if (qty > 0) usages.push({ tape_item_id: tapeItemID, tape_item_name: tapeItemName, quantity: qty });
    setDraft({ ...draft, tape_usages: usages });
  };
  const getUsageQty = (tapeItemID: number) =>
    draft.tape_usages.find(u => u.tape_item_id === tapeItemID)?.quantity ?? 0;

  // --- テープ素材操作 ---
  const openTapeCreate = () => { setEditingTape(null); setTapeDraft({ name: "", stock_count: 0, sort_order: 0, disabled: false }); setShowTapeForm(true); };
  const openTapeEdit = (t: TapeItem) => { setEditingTape(t); setTapeDraft({ name: t.name, stock_count: t.stockCount, sort_order: t.sortOrder, disabled: t.disabled }); setShowTapeForm(true); };
  const saveTape = async () => {
    if (editingTape) { await repo.tapeItemUpdate(editingTape.id, tapeDraft); }
    else { await repo.tapeItemCreate(tapeDraft); }
    setShowTapeForm(false);
    repo.tapeItemList().then(setTapeItems);
  };

  if (myself?.slack?.id && myself.slack.id !== "xxx" && !isTapingManager(myself)) return null;

  return (
    <Layout>
      <div className="pb-24">
        {/* タブ */}
        <div className="flex border-b">
          {(["menu", "tape"] as const).map(tab => (
            <button
              key={tab}
              className={`flex-1 py-3 text-sm font-medium border-b-2 -mb-px transition-colors ${
                activeTab === tab ? "border-blue-700 text-blue-700" : "border-transparent text-gray-400"
              }`}
              onClick={() => setActiveTab(tab)}
            >{tab === "menu" ? "施術メニュー" : "テープ素材"}</button>
          ))}
        </div>

        {/* 施術メニューリスト */}
        {activeTab === "menu" && (
          <div className="divide-y divide-gray-100">
            {items.map(item => (
              <button
                key={item.id}
                className={`w-full flex items-center px-4 py-4 text-left active:bg-gray-50 ${item.disabled ? "opacity-40" : ""}`}
                onClick={() => openEdit(item)}
              >
                <div className="flex-1 min-w-0 pr-2">
                  <div className="text-sm font-medium">{item.name}</div>
                  <div className="text-xs text-gray-400 mt-0.5 truncate">
                    ¥{item.price}
                    {item.tapeUsages?.length > 0 && (
                      <span className="ml-2">{item.tapeUsages.map(u => `${u.tape_item_name}×${u.quantity}`).join(" · ")}</span>
                    )}
                    {item.notes && <span className="ml-2 text-gray-300">{item.notes}</span>}
                  </div>
                </div>
                <ChevronRightIcon className="w-4 h-4 text-gray-300 flex-shrink-0" />
              </button>
            ))}
            {items.length === 0 && (
              <div className="py-16 text-center text-sm text-gray-400">メニューがありません</div>
            )}
          </div>
        )}

        {/* テープ素材リスト */}
        {activeTab === "tape" && (
          <div className="divide-y divide-gray-100">
            {tapeItems.map(t => (
              <button
                key={t.id}
                className={`w-full flex items-center px-4 py-4 text-left active:bg-gray-50 ${t.disabled ? "opacity-40" : ""}`}
                onClick={() => openTapeEdit(t)}
              >
                <div className="flex-1 min-w-0 pr-2">
                  <div className="text-sm font-medium">{t.name}</div>
                  {t.stockCount > 0 && (
                    <div className="text-xs text-gray-400 mt-0.5">基本ストック {t.stockCount}本</div>
                  )}
                </div>
                <ChevronRightIcon className="w-4 h-4 text-gray-300 flex-shrink-0" />
              </button>
            ))}
            {tapeItems.length === 0 && (
              <div className="py-16 text-center text-sm text-gray-400">テープ素材がありません</div>
            )}
          </div>
        )}
      </div>

      {/* 固定下部: 追加ボタン */}
      <div className="fixed left-0 bottom-0 w-full px-4 py-4 bg-white border-t border-gray-100">
        <button
          className="w-full bg-blue-700 text-white py-3 rounded-xl text-sm font-medium"
          onClick={activeTab === "menu" ? openCreate : openTapeCreate}
        >+ {activeTab === "menu" ? "施術メニューを追加" : "テープ素材を追加"}</button>
      </div>

      {/* 施術メニュー フォーム — モーダル */}
      {showForm && (
        <>
          <div className="fixed inset-0 bg-black/40 z-40" onClick={() => setShowForm(false)} />
          <div className="fixed inset-0 z-50 flex items-center justify-center px-4">
            <div className="bg-white rounded-2xl shadow-xl w-full max-w-md flex flex-col max-h-[88vh]">
              <div className="px-5 py-4 flex-shrink-0 border-b border-gray-100">
                <h2 className="text-base font-semibold">{editing ? "メニュー編集" : "メニュー追加"}</h2>
              </div>
              <div className="overflow-y-auto px-5 py-4 space-y-4 flex-1">
                <div>
                  <label className="block text-xs text-gray-500 mb-1">名称 <span className="text-red-400">*</span></label>
                  <input type="text"
                    className="w-full border border-gray-200 rounded-xl px-3 py-3 text-sm"
                    value={draft.name}
                    onChange={e => setDraft({ ...draft, name: e.target.value })} />
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="block text-xs text-gray-500 mb-1">単価目安（円）</label>
                    <input type="number"
                      className="w-full border border-gray-200 rounded-xl px-3 py-3 text-sm"
                      value={draft.price}
                      onChange={e => setDraft({ ...draft, price: Number(e.target.value) })} />
                  </div>
                  <div>
                    <label className="block text-xs text-gray-500 mb-1">表示順</label>
                    <input type="number"
                      className="w-full border border-gray-200 rounded-xl px-3 py-3 text-sm"
                      value={draft.sort_order}
                      onChange={e => setDraft({ ...draft, sort_order: Number(e.target.value) })} />
                  </div>
                </div>
                {tapeItems.filter(t => !t.disabled).length > 0 && (
                  <div>
                    <label className="block text-xs text-gray-500 mb-2">テープ使用量（本）</label>
                    <div className="space-y-2">
                      {tapeItems.filter(t => !t.disabled).map(t => (
                        <div key={t.id} className="flex items-center border border-gray-200 rounded-xl px-3 py-2.5">
                          <span className="text-sm flex-1">{t.name}</span>
                          <input
                            type="number" step="0.5" min="0"
                            className="w-14 text-sm text-right bg-transparent border-0 outline-none"
                            value={getUsageQty(t.id)}
                            onChange={e => setUsageQty(t.id, t.name, Number(e.target.value))}
                          />
                          <span className="text-xs text-gray-400 ml-1">本</span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
                <div>
                  <label className="block text-xs text-gray-500 mb-1">備考</label>
                  <input type="text"
                    className="w-full border border-gray-200 rounded-xl px-3 py-3 text-sm"
                    value={draft.notes}
                    onChange={e => setDraft({ ...draft, notes: e.target.value })} />
                </div>
                <label className="flex items-center space-x-3 py-1 cursor-pointer">
                  <input type="checkbox" className="w-5 h-5 rounded" checked={draft.disabled}
                    onChange={e => setDraft({ ...draft, disabled: e.target.checked })} />
                  <span className="text-sm text-gray-600">無効にする</span>
                </label>
              </div>
              <div className="px-5 pb-5 pt-3 flex-shrink-0 space-y-2 border-t border-gray-100">
                <button
                  className="w-full py-3 bg-blue-700 text-white font-medium rounded-xl text-sm disabled:opacity-40"
                  onClick={saveMenu}
                  disabled={!draft.name}
                >保存する</button>
                {editing && (
                  <button
                    className="w-full py-2.5 text-sm text-red-500"
                    onClick={() => {
                      if (!confirm(`「${editing.name}」を削除しますか？`)) return;
                      repo.menuDelete(editing.id)
                        .then(() => repo.menuList().then(setItems))
                        .then(() => setShowForm(false));
                    }}
                  >削除する</button>
                )}
              </div>
            </div>
          </div>
        </>
      )}

      {/* テープ素材 フォーム — モーダル */}
      {showTapeForm && (
        <>
          <div className="fixed inset-0 bg-black/40 z-40" onClick={() => setShowTapeForm(false)} />
          <div className="fixed inset-0 z-50 flex items-center justify-center px-4">
            <div className="bg-white rounded-2xl shadow-xl w-full max-w-sm">
              <div className="px-5 py-4 border-b border-gray-100">
                <h2 className="text-base font-semibold">{editingTape ? "テープ素材編集" : "テープ素材追加"}</h2>
              </div>
              <div className="px-5 py-4 space-y-4">
                <div>
                  <label className="block text-xs text-gray-500 mb-1">名称 <span className="text-red-400">*</span></label>
                  <input type="text"
                    className="w-full border border-gray-200 rounded-xl px-3 py-3 text-sm"
                    value={tapeDraft.name}
                    onChange={e => setTapeDraft({ ...tapeDraft, name: e.target.value })} />
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="block text-xs text-gray-500 mb-1">基本ストック（本）</label>
                    <input type="number" step="0.5" min="0"
                      className="w-full border border-gray-200 rounded-xl px-3 py-3 text-sm"
                      value={tapeDraft.stock_count}
                      onChange={e => setTapeDraft({ ...tapeDraft, stock_count: Number(e.target.value) })} />
                  </div>
                  <div>
                    <label className="block text-xs text-gray-500 mb-1">表示順</label>
                    <input type="number"
                      className="w-full border border-gray-200 rounded-xl px-3 py-3 text-sm"
                      value={tapeDraft.sort_order}
                      onChange={e => setTapeDraft({ ...tapeDraft, sort_order: Number(e.target.value) })} />
                  </div>
                </div>
                <label className="flex items-center space-x-3 py-1 cursor-pointer">
                  <input type="checkbox" className="w-5 h-5 rounded" checked={tapeDraft.disabled}
                    onChange={e => setTapeDraft({ ...tapeDraft, disabled: e.target.checked })} />
                  <span className="text-sm text-gray-600">無効にする</span>
                </label>
              </div>
              <div className="px-5 pb-5 pt-3 space-y-2 border-t border-gray-100">
                <button
                  className="w-full py-3 bg-blue-700 text-white font-medium rounded-xl text-sm disabled:opacity-40"
                  onClick={saveTape}
                  disabled={!tapeDraft.name}
                >保存する</button>
                {editingTape && (
                  <button
                    className="w-full py-2.5 text-sm text-red-500"
                    onClick={() => {
                      if (!confirm(`「${editingTape.name}」を削除しますか？`)) return;
                      repo.tapeItemDelete(editingTape.id)
                        .then(() => repo.tapeItemList().then(setTapeItems))
                        .then(() => setShowTapeForm(false));
                    }}
                  >削除する</button>
                )}
              </div>
            </div>
          </div>
        </>
      )}
    </Layout>
  );
}
