package azdo

type agentResponseEnvelope struct {
	Count  int     `json:"count"`
	Agents []Agent `json:"value"`
}

type Agent struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Size    int    `json:"size"`
	Version string `json:"version"`
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"`
	poolID  int
}
