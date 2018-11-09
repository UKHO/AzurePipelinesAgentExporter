package main

type agentResponseEnvelope struct {
	Count  int     `json:"count"`
	Agents []agent `json:"value"`
}

type agent struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Size    int    `json:"size"`
	Version string `json:"version"`
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"`
	poolID  int
}
