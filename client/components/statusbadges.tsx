import { MemberStatus } from "../models/Member";

export default function StatusBadges({
  member,
  admin = false, // 見ているひとがadmin
  size = "text-xs"
}) {
  const {slack, injury} = member;
  const badges = [];
  if (slack.deleted) badges.push(
    <span key="is_deleted"
      className={`px-2 inline-flex leading-5 font-semibold rounded-full bg-gray-200 text-gray-800 ${size}`}>
      引退
    </span>
  );
  if (injury) badges.push(
    <span key="is_injured"
      className={`px-2 inline-flex leading-5 font-semibold rounded-full bg-red-600 text-white ${size}`}>
      怪我
    </span>
  );

  if (admin) {
    switch (member.status) {
    case MemberStatus.Limited:
      badges.push(
        <span key="is_limited"
          className={`px-2 inline-flex leading-5 font-semibold rounded-full bg-green-500 text-white ${size}`}>
            練習外
        </span>
      ); break;
    case MemberStatus.Inactive:
      badges.push(
        <span key="is_inactive"
          className={`px-2 inline-flex leading-5 font-semibold rounded-full bg-gray-600 text-white ${size}`}>
          休眠
        </span>
      ); break;
    }
  }

  return <>{badges}</>;
}

