import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import Layout from "../../components/layout";
import Taping from "../../models/Taping";
import TeamEvent from "../../models/TriaxEvent";
import TapingRepo from "../../repository/TapingRepo";
import { useAppContext } from "../context";

function isTapingManager(myself: any): boolean {
  if (!myself?.slack) return false;
  if (myself.slack.is_admin) return true;
  return !!(myself.slack.profile?.title?.match(/trainer/i));
}

export default function TapingOverview() {
  const { myself } = useAppContext();
  const navigate = useNavigate();
  const repo = useMemo(() => new TapingRepo(), []);
  const [events, setEvents] = useState<TeamEvent[]>([]);
  const [selectedEventID, setSelectedEventID] = useState<string>("");
  const [tapings, setTapings] = useState<Taping[]>([]);

  // 権限チェック: placeholder(id="xxx") のうちは待つ
  useEffect(() => {
    if (!myself?.slack?.id || myself.slack.id === "xxx") return;
    if (!isTapingManager(myself)) {
      navigate({ to: "/" });
    }
  }, [myself, navigate]);

  useEffect(() => {
    repo.listEvents().then(evs => {
      setEvents(evs);
      if (evs.length > 0) setSelectedEventID(evs[0].google.id);
    });
  }, [repo]);

  useEffect(() => {
    if (!selectedEventID) return;
    repo.listRequests(selectedEventID).then(setTapings);
  }, [selectedEventID, repo]);

  // MemberID 別に集計
  const byMember = tapings.reduce<Record<string, Taping[]>>((acc, t) => {
    (acc[t.memberID] ||= []).push(t);
    return acc;
  }, {});

  const totalPrice = tapings.reduce((s, t) => s + t.price, 0);
  const totalRolls = tapings.reduce((s, t) => s + t.estimatedRolls, 0);

  if (myself?.slack && !isTapingManager(myself)) return null;

  return (
    <Layout>
      <div className="px-4 py-6 max-w-2xl mx-auto">
        <h1 className="text-xl font-bold mb-4">テーピングリクエスト一覧</h1>

        {/* イベントセレクト */}
        <div className="mb-4">
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

        {/* サマリ */}
        {tapings.length > 0 && (
          <div className="grid grid-cols-3 gap-3 mb-4">
            <div className="bg-blue-50 rounded-lg p-3 text-center">
              <div className="text-2xl font-bold text-blue-700">{Object.keys(byMember).length}</div>
              <div className="text-xs text-gray-500 mt-1">申請人数</div>
            </div>
            <div className="bg-green-50 rounded-lg p-3 text-center">
              <div className="text-2xl font-bold text-green-700">¥{totalPrice.toLocaleString()}</div>
              <div className="text-xs text-gray-500 mt-1">合計金額</div>
            </div>
            <div className="bg-orange-50 rounded-lg p-3 text-center">
              <div className="text-2xl font-bold text-orange-700">{totalRolls.toFixed(1)}</div>
              <div className="text-xs text-gray-500 mt-1">推定テープ（本）</div>
            </div>
          </div>
        )}

        {/* メンバー別リスト */}
        {Object.entries(byMember).length === 0 ? (
          <div className="text-center text-gray-400 py-8">リクエストはありません</div>
        ) : (
          <div className="space-y-3">
            {Object.entries(byMember).map(([memberID, items]) => (
              <div key={memberID} className="border border-gray-200 rounded-lg p-3">
                <div className="flex justify-between items-center mb-2">
                  <span className="text-xs text-gray-500 font-mono">{memberID}</span>
                  <span className="text-sm font-medium">
                    ¥{items.reduce((s, t) => s + t.price, 0).toLocaleString()}
                  </span>
                </div>
                <ul className="space-y-1">
                  {items.map((t, i) => (
                    <li key={i} className="flex justify-between text-sm">
                      <span>{t.menuItemName}</span>
                      <span className="text-gray-500">¥{t.price}</span>
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </div>
        )}
      </div>
    </Layout>
  );
}
