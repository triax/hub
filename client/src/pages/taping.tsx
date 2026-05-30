import { useNavigate } from "@tanstack/react-router";
import { useEffect, useMemo, useState } from "react";
import Layout from "../../components/layout";
import { isTapingManager } from "../utils/tapingAuth"; // マスタ管理ボタンの表示判定にのみ使用
import Taping from "../../models/Taping";
import TapeItem from "../../models/TapeItem";
import Member from "../../models/Member";
import TeamEvent from "../../models/TriaxEvent";
import TapingRepo from "../../repository/TapingRepo";
import { MemberCache } from "../../repository/MemberRepo";
import { useAppContext } from "../context";


export default function TapingOverview() {
  const { myself } = useAppContext();
  const navigate = useNavigate();
  const repo = useMemo(() => new TapingRepo(), []);
  const year = new Date().getFullYear();

  const [yearTapings, setYearTapings] = useState<Taping[]>([]);
  const [tapeItems, setTapeItems] = useState<TapeItem[]>([]);
  const [upcomingEvents, setUpcomingEvents] = useState<TeamEvent[]>([]);
  const [upcomingTapings, setUpcomingTapings] = useState<Taping[]>([]);

  useEffect(() => {
    if (!myself?.slack?.id || myself.slack.id === "xxx") return;
    repo.listRequests(undefined, year).then(setYearTapings);
    repo.tapeItemList().then(setTapeItems);
    repo.listEvents().then(evs => {
      const upcoming = evs.filter(ev => ev.google.start_time > Date.now());
      setUpcomingEvents(upcoming);
      // 今後のイベントのリクエストを全取得
      Promise.all(upcoming.map(ev => repo.listRequests(ev.google.id)))
        .then(results => setUpcomingTapings(results.flat()));
    });
  }, [myself, navigate, repo, year]);

  const activeTapeItems = useMemo(() => tapeItems.filter(t => !t.disabled), [tapeItems]);

  // --- 費用集計（年度累計） ---
  const costByMember = yearTapings.reduce<Record<string, { price: number; count: number }>>((acc, t) => {
    if (!acc[t.memberID]) acc[t.memberID] = { price: 0, count: 0 };
    acc[t.memberID].price += t.price;
    acc[t.memberID].count += 1;
    return acc;
  }, {});
  const totalYearCost = yearTapings.reduce((s, t) => s + t.price, 0);

  // --- テープ在庫状況（今後のイベント） ---
  const upcomingByTape = upcomingTapings.reduce<Record<string, number>>((acc, t) => {
    for (const u of t.tapeUsages ?? []) {
      acc[u.tape_item_name] = (acc[u.tape_item_name] ?? 0) + u.quantity;
    }
    return acc;
  }, {});

  return (
    <Layout>
      <div>
        {/* 費用集計 */}
        <div className="mb-8">
          <div className="border-b mb-2 pb-1 flex justify-between items-baseline">
            <span className="font-semibold text-sm">費用集計（{year}年度）</span>
            <span className="text-sm text-gray-500">合計 ¥{totalYearCost.toLocaleString()}</span>
          </div>
          {Object.keys(costByMember).length === 0 ? (
            <div className="text-sm text-gray-400 py-4 text-center">データがありません</div>
          ) : (
            <div className="divide-y">
              {Object.entries(costByMember)
                .sort((a, b) => b[1].price - a[1].price)
                .map(([memberID, { price, count }]) => (
                  <MemberCostRow key={memberID} memberID={memberID} price={price} count={count} />
                ))}
            </div>
          )}
        </div>

        {/* テープ在庫状況 */}
        <div>
          <div className="border-b mb-2 pb-1 flex justify-between items-baseline">
            <span className="font-semibold text-sm">テープ在庫状況</span>
            <span className="text-xs text-gray-400">今後 {upcomingEvents.length} イベントの申請より</span>
          </div>
          {activeTapeItems.length === 0 ? (
            <div className="text-sm text-gray-400 py-4 text-center">テープ素材が未登録です</div>
          ) : (
            <div className="divide-y">
              {activeTapeItems.map(item => {
                const needed = upcomingByTape[item.name] ?? 0;
                const stock = item.stockCount;
                const shortage = stock > 0 && needed > stock;
                return (
                  <div key={item.id} className="flex items-center py-2 text-sm">
                    <div className="flex-1">{item.name}</div>
                    <div className="text-right space-x-3">
                      <span className="text-gray-500">必要 {needed.toFixed(1)}本</span>
                      {stock > 0 ? (
                        <>
                          <span className="text-gray-400">/ ストック {stock}本</span>
                          {shortage
                            ? <span className="text-red-500 font-medium">⚠ 不足</span>
                            : <span className="text-green-600">✓</span>
                          }
                        </>
                      ) : (
                        <span className="text-gray-300">ストック未設定</span>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
          {activeTapeItems.some(t => t.stockCount === 0) && (
            <div className="mt-2 text-xs text-gray-400">
              ストック本数は
              <button className="underline mx-1" onClick={() => navigate({ to: "/taping/master" })}>テープ素材マスタ</button>
              で設定できます。
            </div>
          )}
        </div>
      </div>

      <div className="px-4 py-4 fixed left-0 bottom-0 w-full">
        <div
          className="text-center border border-blue-600 text-blue-600 p-2 rounded-md shadow-md shadow-gray-300 bg-white text-sm font-medium cursor-pointer"
          onClick={() => navigate({ to: "/taping/master" })}
        >マスタ管理</div>
      </div>
    </Layout>
  );
}

function MemberCostRow({ memberID, price, count }: { memberID: string; price: number; count: number }) {
  const [member, setMember] = useState<Member>(null);
  useEffect(() => { new MemberCache().get(memberID).then(setMember); }, [memberID]);
  const name = member?.slack?.profile?.display_name || member?.slack?.profile?.real_name || memberID;
  return (
    <div className="flex items-center py-2 text-sm">
      {member?.slack?.profile?.image_512 ? (
        <div className="w-6 h-6 rounded-full overflow-hidden flex-shrink-0 mr-2">
          <img src={member.slack.profile.image_512} alt={name} className="w-full h-full object-cover" />
        </div>
      ) : <div className="w-6 mr-2" />}
      <div className="flex-1">{name}</div>
      <div className="text-gray-400 text-xs mr-3">{count}件</div>
      <div className="font-medium">¥{price.toLocaleString()}</div>
    </div>
  );
}
