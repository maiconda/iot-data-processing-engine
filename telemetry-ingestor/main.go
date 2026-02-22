package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/nats-io/nats.go"
)

type Telemetria struct {
	Imei       string  `json:"imei"`
	Latitude   float64 `json:"lat"`
	Longitude  float64 `json:"lng"`
	Velocidade int     `json:"velocidade"`
	Date       string  `json:"date"`
}

var nc *nats.Conn
var js nats.JetStreamContext

func receberTelemetria(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Apenas POST", http.StatusMethodNotAllowed)
		return
	}

	var dado Telemetria
	err := json.NewDecoder(r.Body).Decode(&dado)

	if err != nil {
		fmt.Println("❌ ERRO AO LER JSON:", err)

		http.Error(w, "JSON Inválido", http.StatusBadRequest)
		return
	}

	payloadJSON, errMarshal := json.Marshal(dado)
	if errMarshal != nil {
		fmt.Println("❌ Erro ao serializar JSON:", errMarshal)
		http.Error(w, "Erro Interno ao processar dados", http.StatusInternalServerError)
		return
	}

	_, err = js.Publish("iot.telemetria", payloadJSON)

	if err != nil {
		fmt.Println("❌ Erro ao salvar no JetStream:", err)
		http.Error(w, "Erro Interno no Broker", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte(`{"status": "success"}`))

	if err != nil {
		fmt.Println("⚠️ Erro ao enviar resposta HTTP ao cliente:", err)
		return
	}
}

func main() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}

	var natsErr error
	nc, natsErr = nats.Connect(natsURL)
	if natsErr != nil {
		log.Fatal("🚨 ERRO FATAL: Não foi possível conectar ao NATS: ", natsErr)
	}
	defer nc.Close()

	js, natsErr = nc.JetStream()
	if natsErr != nil {
		log.Fatal("🚨 ERRO FATAL: Não foi possível iniciar o JetStream: ", natsErr)
	}

	streamName := "STREAM_IOT"

	_, errStream := js.StreamInfo(streamName)

	if errStream != nil {
		_, errStream = js.AddStream(&nats.StreamConfig{
			Name:     streamName,
			Subjects: []string{"iot.*"},
		})

		if errStream != nil {
			log.Fatal("🚨 ERRO FATAL: Falha ao criar o Stream no NATS: ", errStream)
		}
	}

	http.HandleFunc("/telemetria", receberTelemetria)

	porta := os.Getenv("PORT")
	if porta == "" {
		porta = "8080"
	}

	err := http.ListenAndServe(":"+porta, nil)
	if err != nil {
		return
	}
}
