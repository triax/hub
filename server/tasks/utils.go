package tasks

import (
	"fmt"
	"net/http"
	"time"

	"github.com/triax/hub/server"
)

// defineTimeRangeByRequest
// reqの中に、from/toの情報が無ければ「今から24時間以内」を返す.
// reqの中に、from/toの情報が有れば、「本日の」指定範囲内を返す.
func defineTimeRangeByRequest(n time.Time, req *http.Request) (f time.Time, t time.Time, err error) {
	n = n.In(server.ServiceLocation)
	ft, err := time.Parse("15:04", req.URL.Query().Get("from"))
	if err != nil {
		return f, t, fmt.Errorf("failed to parse `from` parameter: %v", err)
	}
	tt, err := time.Parse("15:04", req.URL.Query().Get("to"))
	if err != nil {
		return f, t, fmt.Errorf("failed to parse `to` parameter: %v", err)
	}
	f = time.Date(n.Year(), n.Month(), n.Day(), ft.Hour(), ft.Minute(), 0, 0, server.ServiceLocation)
	t = time.Date(n.Year(), n.Month(), n.Day(), tt.Hour(), tt.Minute(), 0, 0, server.ServiceLocation)
	return f, t, nil
}
