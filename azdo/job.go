package azdo

import "time"

type jobResponseEnvelope struct {
	Count int   `json:"count"`
	Jobs  []Job `json:"value"`
}

type Job struct {
	RequestID   int       `json:"requestId"`
	Name        string    `json:"name"`
	QueueTime   time.Time `json:"queueTime"`
	AssignTime  time.Time `json:"assignTime"`
	ReceiveTime time.Time `json:"receiveTime"`
	FinishTime  time.Time `json:"finishTime"`
	Result      string    `json:"result"`
	JobID       string    `json:"jobId"`
	PlanType    string    `json:"planType"`
}
