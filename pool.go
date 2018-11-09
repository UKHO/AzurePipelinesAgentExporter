package main

type poolResponseEnvelope struct {
	Count int    `json:"count"`
	Pools []pool `json:"value"`
}

type pool struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Size     int    `json:"size"`
	IsHosted bool   `json:"isHosted"`
}
