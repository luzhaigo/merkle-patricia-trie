package proxy

type Config struct {
	Port      int
	Impl      string
	MaxHops   int
	AdminPort int
}
