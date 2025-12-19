package main

type Configuration struct {
	Key      string `json:"key"`
	Chatid   string `json:"chatid"`
	RabbitMQ struct {
		Host     string    `json:"Host"`
		Port        int    `json:"Port"`
		Username    string `json:"Username"`
		Password    string `json:"Password"`
		UseRabbitMQ bool   `json:"UseRabbitMQ"`
	} `json:"RabbitMQ"`
}

type RequestBody struct {
	Message string `json:"message"`
}

type FileMessage struct {
	ContentType string
	Data        []byte
	FileName string
}