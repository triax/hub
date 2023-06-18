import EventRSVPButtonsRow from "./RSVPButtons";

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

function EventParticipantsRow({ row }) {
  return <div className="flex -space-x-1 overflow-hidden">
    {row.map(([id, p]: [string, any]) => (
      <div
        key={id}
        className="inline-block h-4 w-4 rounded-full ring-1 ring-white"
        style={{backgroundImage: `url(${p.picture})`, backgroundSize: 'cover'}}
      />
    ))}
  </div>;
}

function EventParticipantsIcons({pats, onClick = () => {}}) {
  const entries = Object.entries(pats)
    .filter(([_, p]: [string, any]) => p.type == 'join' || p.type == 'join_late') || [];
  if (entries.length == 0) return null;
  const chunk = 25;
  const rows = entries.reduce((ctx, e, i) => {
    if (i % chunk == 0) ctx.push(entries.slice(i, i+chunk));
    return ctx;
  }, []);
  return (
    <div onClick={onClick} >
      {rows.map((row, i) => <EventParticipantsRow key={i} row={row} />)}
    </div>
  )
}

export function EventRow({ event, myself, submit, setModalEvent, router }) {
  const pats = event.participations;
  const answer = pats[myself.slack.id] || {};
  const id = event.google.id.replace(/@google\.com$/, "");
  if (event.google?.title?.match(/#ignore$/)) return null;
  return (
    <div className={"px-0 py-4 " + (event.google.start_time < Date.now() ? "bg-slate-400" : "")}>
      <div onClick={() => router.push(`/events/${id}`)}>
        <EventDateTime timestamp={event.google.start_time} />
        <h3 className="text-gray-900 text-sm font-bold">{event.google.title}</h3>
        {event.google.start_time < Date.now() ? null : <>
          <EventLocation location={event.google.location} />
          <EventParticipantsIcons pats={pats} />
        </>}
      </div>
      {event.google.start_time < Date.now() ? null : <EventRSVPButtonsRow
        event={event}
        answer={answer}
        setModalEvent={setModalEvent}
        submit={submit}
      />}
    </div>
  );
}

export function EventList({children}) {
  return <div className="divide-y grid grid-cols-1">{children}</div>;
}