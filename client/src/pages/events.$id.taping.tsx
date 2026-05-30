import { useNavigate, useParams } from "@tanstack/react-router";
import { useEffect, useMemo, useState } from "react";
import Layout from "../../components/layout";
import Taping from "../../models/Taping";
import TeamEvent from "../../models/TriaxEvent";
import TapingRepo from "../../repository/TapingRepo";
import TeamEventRepo from "../../repository/EventRepo";
import { useAppContext } from "../context";

function isTapingManager(myself: any): boolean {
  if (!myself?.slack?.id || myself.slack.id === "xxx") return false;
  if (myself.slack.is_admin) return true;
  return !!(myself.slack.profile?.title?.match(/trainer/i));
}

export default function EventTaping() {
  const { myself } = useAppContext();
  const { id } = useParams({ strict: false });
  const navigate = useNavigate();
  const tapingRepo = useMemo(() => new TapingRepo(), []);
  const eventRepo = useMemo(() => new TeamEventRepo(), []);
  const [event, setEvent] = useState<TeamEvent | null>(null);
  const [tapings, setTapings] = useState<Taping[]>([]);
  const [myTapings, setMyTapings] = useState<Taping[]>([]);
  const isManager = isTapingManager(myself);

  useEffect(() => {
    if (!id) return;
    eventRepo.get(id).then(setEvent);
    tapingRepo.getMyRequest(id).then(setMyTapings);
    if (isManager) tapingRepo.listRequests(id).then(setTapings);
  }, [id, isManager, tapingRepo, eventRepo]);

  const byMember = tapings.reduce<Record<string, Taping[]>>((acc, t) => {
    (acc[t.memberID] ||= []).push(t);
    return acc;
  }, {});
  const totalPrice = tapings.reduce((s, t) => s + t.price, 0);
  const totalRolls = tapings.reduce((s, t) => s + t.estimatedRolls, 0);

  return (
    <Layout>
      <div className="px-4 py-6 max-w-2xl mx-auto">
        <div className="mb-4">
          <button
            className="text-sm text-blue-600 hover:underline"
            onClick={() => navigate({ to: `/events/${id}` })}
          >← {event?.google?.title ?? "イベント"}</button>
        </div>

        <h1 className="text-xl font-bold mb-1">テーピングリクエスト</h1>
        {event && (
          <p className="text-sm text-gray-500 mb-4">
            {new Date(event.google.start_time).toLocaleDateString("ja-JP")} {event.google.title}
          </p>
        )}

        {/* 自分のリクエスト */}
        <div className="mb-6">
          <div className="flex items-center justify-between mb-2">
            <h2 className="text-sm font-medium text-gray-700">自分のリクエスト</h2>
            <button
              className="text-xs bg-blue-700 text-white px-3 py-1 rounded-md"
              onClick={() => navigate({ to: "/taping/request" })}
            >{myTapings.length > 0 ? "変更する" : "リクエストする"}</button>
          </div>
          {myTapings.length === 0 ? (
            <p className="text-sm text-gray-400">まだリクエストがありません</p>
          ) : (
            <ul className="space-y-1 text-sm border border-gray-200 rounded-lg p-3">
              {myTapings.map((t, i) => (
                <li key={i} className="flex justify-between">
                  <span>{t.menuItemName}</span>
                  <span className="text-gray-500">¥{t.price}</span>
                </li>
              ))}
              <li className="flex justify-between font-medium pt-1 border-t border-gray-100">
                <span>合計</span>
                <span>¥{myTapings.reduce((s, t) => s + t.price, 0).toLocaleString()}</span>
              </li>
            </ul>
          )}
        </div>

        {/* 全体集計（admin/trainer のみ） */}
        {isManager && (
          <>
            <h2 className="text-sm font-medium text-gray-700 mb-2">全体集計</h2>
            {tapings.length === 0 ? (
              <p className="text-sm text-gray-400">リクエストはありません</p>
            ) : (
              <>
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
              </>
            )}
          </>
        )}
      </div>
    </Layout>
  );
}
