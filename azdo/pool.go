package azdo

type poolResponseEnvelope struct {
	Count int    `json:"count"`
	Pools []Pool `json:"value"`
}

type Pool struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Size     int    `json:"size"`
	IsHosted bool   `json:"isHosted"`
}
