import { useEffect, useMemo, useState } from "react";
import Layout from "../../components/layout";
import TapingMenuItem from "../../models/TapingMenuItem";
import TeamEvent from "../../models/TriaxEvent";
import TapingRepo from "../../repository/TapingRepo";

export default function TapingRequest() {
  const repo = useMemo(() => new TapingRepo(), []);
  const [menuItems, setMenuItems] = useState<TapingMenuItem[]>([]);
  const [events, setEvents] = useState<TeamEvent[]>([]);
  // URL の ?event= があれば初期値として使う
  const initialEventID = useMemo(() => new URLSearchParams(window.location.search).get("event") ?? "", []);
  const [selectedEventID, setSelectedEventID] = useState<string>(initialEventID);
  const [selectedIDs, setSelectedIDs] = useState<Set<number>>(new Set());
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
      setSelectedIDs(new Set(tapings.map(t => t.menuItemID)));
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
      await repo.submitRequest(selectedEventID, Array.from(selectedIDs));
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
        </div>

        {submitted && (
          <div className="mb-4 p-3 bg-green-50 border border-green-200 rounded-md text-sm text-green-700">
            {selectedEvent?.google.title} へのリクエストを送信しました。
          </div>
        )}

        <button
          className="w-full bg-blue-700 text-white py-3 rounded-md font-medium disabled:opacity-50"
          onClick={submit}
          disabled={submitting || selectedIDs.size === 0}
        >
          {submitting ? "送信中..." : "送信する"}
        </button>
      </div>
    </Layout>
  );
}
