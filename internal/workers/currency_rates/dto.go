package currency_rates

type cbrResponse struct {
	Date   string `json:"Date"`
	Valute map[string]struct {
		CharCode string  `json:"CharCode"`
		Nominal  int     `json:"Nominal"`
		Value    float64 `json:"Value"`
	} `json:"Valute"`
}
