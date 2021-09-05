
function cn(...classes): string {
  return classes.filter(Boolean).join(' ');
}

const weekday = {
  0: "日", 1: "月", 2: "火", 3: "水", 4: "木", 5: "金", 6: "土"
}

export function EventDateTime({ timestamp, className = "text-xs text-gray-500" }) {
  const date = new Date(timestamp);
  return (
    <div className={className} >
      {date.getMonth() + 1}月 {date.getDate()}日（{weekday[date.getDay()]}） {date.getHours()}:{("0" + date.getMinutes()).slice(-2)}
    </div>
  )
}

export function EventLocation({location}) {
  return (
    <div>
      <div className="text-xs text-gray-400">
        {location}
      </div>
    </div>
  )
}

function EventParticipantsIcons({pats, onClick = () => {}}) {
  const entries = Object.entries(pats)
    .filter(([_, p]: [string, any]) => p.type == 'join' || p.type == 'join_late') || [];
  if (entries.length == 0) return null;
  const maxVisible = 10;
  const visibles = entries.length > maxVisible ? entries.slice(0, maxVisible) : entries;
  const rest = entries.length - visibles.length;
  return (
    <div className="flex" onClick={onClick} >
      <div className="flex -space-x-2">
        {visibles.map(([id, p]: [string, any]) => (
          <img
            key={id} src={p.picture} alt={p.name}
            className="w-6 h-6 rounded-full border-2 border-white"
          />
        ))}
      </div>
      {rest > 0 ? <span className="text-gray-400 text-sm items-center">+{rest}</span> : null}
    </div>
  )
}

export function EventRow({event, myself, submit, setModalEvent}) {
  const pats = JSON.parse(event.participations_json_str || "{}");
  const answer = pats[myself.openid.sub] || {};
  const id = event.google.id.replace(/@google\.com$/, "");
  return (
    <div className="px-0 py-4">
      <div onClick={() => location.href = `/events/${id}`}>
        <EventDateTime timestamp={event.google.start_time} />
        <h3 className="text-gray-900 text-sm font-bold">{event.google.title}</h3>
        <EventLocation location={event.google.location} />
        <EventParticipantsIcons pats={pats} />
      </div>
      <div className="px-0 pt-4 flex items-center">
        <div className="flex">
          {answer.type === undefined ? (
            <div
              className="text-red-600 font-medium text-sm border border-red-600 px-1 py-1 rounded-md"
            ><span>未回答</span></div>
          ) : null}
        </div>
        <div className="flex flex-grow flex-row-reverse">
          <div className="w-60 flex justify-end divide-x font-medium text-gray-400">
            <div className="w-1/3 flex justify-center cursor-pointer"
              onClick={() => setModalEvent(event)}
            >
              <span className={cn(
                'px-1 py-1 rounded-md',
                ['join_late', 'leave_early'].includes(answer.type) ? 'bg-green-400 text-white' : ''
              )}>遅参/早退</span>
            </div>
            <div className="w-1/3 flex justify-center cursor-pointer"
              onClick={() => submit({ event, answer: "absent" })}
            >
              <span className={cn(
                'px-1 py-1 rounded-md',
                answer.type == 'absent' ? 'bg-red-400 text-white' : ''
              )}>不参加</span>
            </div>
            <div className="w-1/3 flex justify-center cursor-pointer"
              onClick={() => submit({ event, answer: "join" })}
            >
              <span className={cn(
                'px-3 py-1 rounded-md',
                answer.type == 'join' ? 'bg-blue-600 text-white' : ''
              )}>参加</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

export function EventList({children}) {
  return <div className="divide-y grid grid-cols-1">{children}</div>;
}