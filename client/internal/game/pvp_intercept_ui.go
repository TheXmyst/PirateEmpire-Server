package game

func (g *Game) confirmIntercept() {
	g.ui.InterceptBusy = true
	g.ui.InterceptError = ""

	go func() {
		defer func() { g.ui.InterceptBusy = false }()

		err := g.api.StartIntercept(g.ui.InterceptAttackerFleetID, g.ui.InterceptTargetFleetID)
		if err != nil {
			g.ui.InterceptError = friendlyError(err)
			return
		}

		g.Log("[PVP] Interception lancée !")
		g.ui.ShowInterceptConfirm = false
		g.loadPlayerData()
		g.loadPvpTargets()
	}()
}
