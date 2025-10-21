package lib

type ClientFilter interface {
	NotifyFailure(ip string)
	CheckBlocked(ip string) bool
	Reset()
}

type Bucket interface {
	GetToken() bool
}
