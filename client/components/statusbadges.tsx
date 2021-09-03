
export default function StatusBadges({
  member,
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

  return <>{badges}</>;
}

