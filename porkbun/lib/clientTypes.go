package lib

type authRequest struct {
	APIKey       string `json:"apikey"`
	SecretAPIKey string `json:"secretapikey"`
	Name         string `json:"name,omitempty"`
	Type         string `json:"type,omitempty"`
	Content      string `json:"content,omitempty"`
	TTL          string `json:"ttl,omitempty"`
	Prio         string `json:"prio,omitempty"`
	Notes        string `json:"notes,omitempty"`
}

type Record struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Type    string `json:"type,omitempty"`
	Content string `json:"content,omitempty"`
	TTL     string `json:"ttl,omitempty"`
	Prio    string `json:"prio,omitempty"`
	Notes   string `json:"notes,omitempty"`
}

type Status struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type recordCreateResp struct {
	Status

	ID int `json:"id"`
}

type recordListResp struct {
	Status

	Records []Record `json:"records"`
}
