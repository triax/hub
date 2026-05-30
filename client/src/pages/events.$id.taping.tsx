import { useNavigate, useParams } from "@tanstack/react-router";
import { useEffect, useMemo, useState } from "react";
import { isTapingManager } from "../utils/tapingAuth";
import Layout from "../../components/layout";
import Taping from "../../models/Taping";
import Member from "../../models/Member";
import TeamEvent from "../../models/TriaxEvent";
import TapingRepo from "../../repository/TapingRepo";
import TeamEventRepo from "../../repository/EventRepo";
import { MemberCache } from "../../repository/MemberRepo";
import { useAppContext } from "../context";


export default function EventTaping() {
  const { myself } = useAppContext();
  const { id } = useParams({ strict: false });
  const navigate = useNavigate();
  const tapingRepo = useMemo(() => new TapingRepo(), []);
  const eventRepo = useMemo(() => new TeamEventRepo(), []);
  const [event, setEvent] = useState<TeamEvent | null>(null);
  const [tapings, setTapings] = useState<Taping[]>([]);

  useEffect(() => {
    if (!myself?.slack?.id || myself.slack.id === "xxx") return;
    if (!isTapingManager(myself)) { navigate({ to: "/" }); return; }
    if (!id) return;
    eventRepo.get(id).then(setEvent);
    tapingRepo.listRequests(id).then(setTapings);
  }, [id, myself, tapingRepo, eventRepo, navigate]);

  const byMember = tapings.reduce<Record<string, Taping[]>>((acc, t) => {
    (acc[t.memberID] ||= []).push(t);
    return acc;
  }, {});

  const totalPrice = tapings.reduce((s, t) => s + t.price, 0);
  const totalRolls  = tapings.reduce((s, t) => s + t.estimatedRolls, 0);

  if (myself?.slack?.id && myself.slack.id !== "xxx" && !isTapingManager(myself)) return null;

  return (
    <Layout>
      <div>
        <h1 className="text-xl text-gray-800 mb-1">{event?.google?.title ?? "…"}</h1>
        <div className="text-sm text-gray-500 mb-4">テーピングリクエスト一覧</div>

        {/* サマリ行 */}
        <div className="flex border-t border-b py-2 space-x-6 text-sm mb-6">
          <div>
            <span className="font-semibold">{Object.keys(byMember).length}</span>
            <span className="text-gray-400 ml-1">人</span>
          </div>
          <div>
            <span className="font-semibold">¥{totalPrice.toLocaleString()}</span>
            <span className="text-gray-400 ml-1">合計</span>
          </div>
          <div>
            <span className="font-semibold">{totalRolls.toFixed(1)}</span>
            <span className="text-gray-400 ml-1">本テープ</span>
          </div>
        </div>

        {/* メンバー別リスト */}
        {Object.keys(byMember).length === 0 ? (
          <div className="text-sm text-gray-400 py-8 text-center">リクエストはありません</div>
        ) : (
          <div className="space-y-5">
            {Object.entries(byMember).map(([memberID, items]) => (
              <MemberTapingRow key={memberID} memberID={memberID} items={items} />
            ))}
          </div>
        )}

        <div className="pt-10 pb-4 text-sm">
          <button
            className="text-gray-400 hover:text-gray-600"
            onClick={() => navigate({ to: `/events/${id}` })}
          >← イベントに戻る</button>
        </div>
      </div>

      {/* 下部固定ボタン */}
      <div className="fixed left-0 bottom-0 w-full px-4 py-4 bg-white border-t border-gray-100">
        <button
          className="w-full border border-blue-600 text-blue-600 py-3 rounded-md font-medium text-sm"
          onClick={() => navigate({ to: `/taping/request?event=${id}` })}
        >テーピングリクエストをする</button>
      </div>
    </Layout>
  );
}

function MemberTapingRow({ memberID, items }: { memberID: string; items: Taping[] }) {
  const [member, setMember] = useState<Member>(null);

  useEffect(() => {
    new MemberCache().get(memberID).then(setMember);
  }, [memberID]);

  const subtotal = items.reduce((s, t) => s + t.price, 0);
  const name = member?.slack?.profile?.display_name
    || member?.slack?.profile?.real_name
    || memberID;

  return (
    <div>
      <div className="flex items-center space-x-2 border-b pb-1 mb-1">
        {member?.slack?.profile?.image_512 ? (
          <div className="w-6 h-6 rounded-full overflow-hidden flex-shrink-0">
            <img
              src={member.slack.profile.image_512}
              alt={name}
              className="w-full h-full object-cover"
            />
          </div>
        ) : null}
        <div className="flex-1 font-medium text-sm">{name}</div>
        <div className="text-sm text-gray-500">¥{subtotal.toLocaleString()}</div>
      </div>
      <div className="divide-y divide-gray-100">
        {items.map((t, i) => (
          <div key={i} className="flex justify-between py-1 text-sm text-gray-700">
            <span>{t.menuItemName}</span>
            <span className="text-gray-400">¥{t.price}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
