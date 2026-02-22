package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Telemetria struct {
	Imei       string  `json:"imei"`
	Latitude   float64 `json:"lat"`
	Longitude  float64 `json:"lng"`
	Velocidade int     `json:"velocidade"`
}

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

	fmt.Printf("➡️ [NATS PUBLISH] Tópico: 'fila.telemetria' | Veículo: %s | Vel: %d km/h\n", dado.Imei, dado.Velocidade)

	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte(`{"status": "ok, enviado para a fila"}`))
	if err != nil {
		return
	}
}

func main() {
	http.HandleFunc("/telemetria", receberTelemetria)
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		return
	}
}
