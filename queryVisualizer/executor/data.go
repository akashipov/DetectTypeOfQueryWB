package executor

type Product struct {
	ID float64 `json:"id"`
}

type Response struct {
	Data struct {
		Products []Product `json:"products"`
	} `json:"data"`
}
