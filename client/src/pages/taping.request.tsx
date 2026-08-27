import { useSearch } from "@tanstack/react-router";
import { useEffect, useMemo, useState } from "react";
import Layout from "../../components/layout";
import Taping from "../../models/Taping";
import TapingMenuItem from "../../models/TapingMenuItem";
import TeamEvent from "../../models/TriaxEvent";
import TapingRepo from "../../repository/TapingRepo";

// 「その他」自由記述の上限。サーバ側（server/api/taping.go）と揃える。
const NOTE_MAX_LEN = 200;

export default function TapingRequest() {
  const repo = useMemo(() => new TapingRepo(), []);
  const [menuItems, setMenuItems] = useState<TapingMenuItem[]>([]);
  const [events, setEvents] = useState<TeamEvent[]>([]);
  // URL の ?event= があれば初期値として使う
  const search = useSearch({ strict: false }) as { event?: string };
  const initialEventID = useMemo(() => search.event ?? "", [search.event]);
  const [selectedEventID, setSelectedEventID] = useState<string>(initialEventID);
  const [selectedIDs, setSelectedIDs] = useState<Set<number>>(new Set());
  const [note, setNote] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [submitted, setSubmitted] = useState(false);

  useEffect(() => {
    Promise.all([repo.menuList(), repo.listEvents()]).then(([items, evs]) => {
      setMenuItems(items.filter(it => !it.disabled));
      setEvents(evs);
      // ?event= 指定がなければ最新イベントをデフォルト選択
      if (!initialEventID && evs.length > 0) setSelectedEventID(evs[0].google.id);
    });
  }, [repo, initialEventID]);

  // イベントが変わったら既存リクエストを読み込む
  useEffect(() => {
    if (!selectedEventID) return;
    setSubmitted(false);
    repo.getMyRequest(selectedEventID).then(tapings => {
      // 「その他」（menuItemID === 0）はチェックボックスに対応しないので選択集合には入れず、
      // 自由記述欄の初期値として復元する。
      setSelectedIDs(new Set(tapings.filter(t => !Taping.isNote(t)).map(t => t.menuItemID)));
      setNote(tapings.find(Taping.isNote)?.note ?? "");
    });
  }, [selectedEventID, repo]);

  const toggle = (id: number) => {
    setSelectedIDs(prev => {
      const next = new Set(prev);
      if (next.has(id)) { next.delete(id); } else { next.add(id); }
      return next;
    });
  };

  const submit = async () => {
    if (!selectedEventID) return;
    setSubmitting(true);
    try {
      await repo.submitRequest(selectedEventID, Array.from(selectedIDs), note.trim());
      setSubmitted(true);
    } finally {
      setSubmitting(false);
    }
  };

  const selectedEvent = events.find(e => e.google.id === selectedEventID);

  return (
    <Layout>
      <div className="px-4 py-6 max-w-lg mx-auto">
        <h1 className="text-xl font-bold mb-4">テーピングリクエスト</h1>

        {/* イベントセレクト */}
        <div className="mb-6">
          <label className="block text-sm font-medium text-gray-700 mb-1">対象イベント</label>
          <select
            className="w-full border border-gray-300 rounded-md p-2 text-sm"
            value={selectedEventID}
            onChange={e => setSelectedEventID(e.target.value)}
          >
            {events.map(ev => (
              <option key={ev.google.id} value={ev.google.id}>
                {new Date(ev.google.start_time).toLocaleDateString("ja-JP")} {ev.google.title}
              </option>
            ))}
          </select>
        </div>

        {/* 部位チェックボックス */}
        <div className="mb-6">
          <label className="block text-sm font-medium text-gray-700 mb-2">
            テープを巻く部位を教えてください <span className="text-red-500">*</span>
          </label>
          <div className="space-y-2">
            {menuItems.map(item => (
              <label key={item.id} className="flex items-center space-x-3 cursor-pointer">
                <input
                  type="checkbox"
                  className="w-5 h-5 rounded border-gray-300"
                  checked={selectedIDs.has(item.id)}
                  onChange={() => toggle(item.id)}
                />
                <span className="text-sm">{item.name}</span>
                {item.price > 0 && (
                  <span className="text-xs text-gray-400">¥{item.price}</span>
                )}
              </label>
            ))}
          </div>

          {/*
            「その他」自由記述。マスタ項目のチェックボックス群（上の space-y-2）の外側に置くことで、
            日本語名ソートの対象外かつ常に末尾、という位置を DOM 構造で保証する。
          */}
          <div className="mt-3">
            <label htmlFor="taping-note" className="block text-sm text-gray-600 mb-1">その他（自由記述）</label>
            <input
              id="taping-note"
              type="text"
              className="w-full border border-gray-300 rounded-md p-2 text-sm"
              placeholder="マスタにない部位・要望があれば記入してください"
              maxLength={NOTE_MAX_LEN}
              value={note}
              onChange={e => setNote(e.target.value)}
            />
          </div>
        </div>

        {submitted && (
          <div className="mb-4 p-3 bg-green-50 border border-green-200 rounded-md text-sm text-green-700">
            {selectedEvent?.google.title} へのリクエストを送信しました。
          </div>
        )}

        <button
          className="w-full bg-blue-700 text-white py-3 rounded-md font-medium disabled:opacity-50"
          onClick={submit}
          disabled={submitting || (selectedIDs.size === 0 && note.trim() === "")}
        >
          {submitting ? "送信中..." : "送信する"}
        </button>
      </div>
    </Layout>
  );
}
