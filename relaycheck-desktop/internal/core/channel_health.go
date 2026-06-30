package core

import (
	"net/http"
)

func (a *App) handleChannelHealthOverview(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	overview, err := a.channelHealthOverview(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

// channelHealthOverview is the *App forwarder for
// channels.Service.ChannelHealthOverview. Wraps the call in cachedRead so the
// host keeps control of cache lifecycle (per channels package contract).
// Converts the channels mirror type back to core ChannelHealthOverview so
// existing callers (tests, handler) are unchanged.
func (a *App) channelHealthOverview(r *http.Request) (ChannelHealthOverview, error) {
	return cachedRead(a, "channel-health-overview", overviewReadCacheTTL, func() (ChannelHealthOverview, error) {
		mirror, err := a.channelsService.ChannelHealthOverview(r.Context())
		if err != nil {
			return ChannelHealthOverview{}, err
		}
		return channelHealthOverviewFromMirror(mirror), nil
	})
}
