package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"
)

type Telemetria struct {
	Imei       string  `json:"imei"`
	Latitude   float64 `json:"lat"`
	Longitude  float64 `json:"lng"`
	Velocidade int     `json:"velocidade"`
	Date       string  `json:"date"`
}

func simularRastreador(imei string) {
	urlDestino := os.Getenv("URL_INGESTOR")

	if urlDestino == "" {
		urlDestino = "http://localhost:8080/telemetria"
	}

	for {
		dados := Telemetria{
			Imei:       imei,
			Latitude:   -27.0 + (rand.Float64() * 2),
			Longitude:  -51.0 + (rand.Float64() * 2),
			Velocidade: rand.Intn(120),
			Date:       time.Now().Format("2006-01-02 15:04:05"),
		}

		payloadJSON, erroJSON := json.Marshal(dados)
		if erroJSON != nil {
			fmt.Println("Erro ao converter para JSON:", erroJSON)
			continue
		}

		resposta, erroHTTP := http.Post(urlDestino, "application/json", bytes.NewBuffer(payloadJSON))

		if erroHTTP != nil {
			fmt.Printf("🔴 [Rastreador %s] Falha de conexão: Servidor offline!\n", imei)
		} else {
			err := resposta.Body.Close()
			if err != nil {
				return
			}
		}

		milisegundos := rand.Intn(3000)
		time.Sleep(time.Duration(milisegundos) * time.Millisecond)
	}
}

func main() {
	for i := 1; i <= 20000; i++ {
		imei := fmt.Sprintf("TRUCK-%04d", i)
		go simularRastreador(imei)
	}
	select {}
}
