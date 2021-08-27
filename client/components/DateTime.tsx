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
