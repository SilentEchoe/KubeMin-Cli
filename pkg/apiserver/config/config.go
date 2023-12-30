package config

type config struct {
	BinAddr string
}

func NewConfig() *config {
	return &config{BinAddr: "0.0.0.0:8000"}
}
