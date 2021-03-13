package azdo

type buildResponseEnvelope struct {
	Count int    `json:"count"`
	Pools []Build `json:"value"`
}

type Build struct {
	buildNumber       string    `json:"buildNumber"`
	finishTime string   `json:"finishTime"`
}
