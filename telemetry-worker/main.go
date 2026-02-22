package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/nats-io/nats.go"
)

type Telemetria struct {
	Imei       string  `json:"imei"`
	Latitude   float64 `json:"lat"`
	Longitude  float64 `json:"lng"`
	Velocidade int     `json:"velocidade"`
	Date       string  `json:"date"`
}

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	natsURL := os.Getenv("NATS_URL")

	if dbURL == "" || natsURL == "" {
		log.Fatal("🚨 ERRO: DATABASE_URL e NATS_URL são obrigatórios")
	}

	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatal("🚨 Erro ao conectar no NATS: ", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		log.Fatal("🚨 Erro ao acessar JetStream: ", err)
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("🚨 Erro ao conectar no banco: ", err)
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {

		}
	}(db)

	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(10)

	sub, err := js.PullSubscribe("iot.telemetria", "worker-grupo-1")
	if err != nil {
		log.Fatal("🚨 Erro ao criar PullSubscribe: ", err)
	}

	fmt.Println("🚀 Worker de Telemetria Iniciado! Aguardando dados...")

	tamanhoLote := 100

	for {
		msgs, err := sub.Fetch(tamanhoLote, nats.MaxWait(2*time.Second))

		if err != nil {
			if errors.Is(err, nats.ErrTimeout) {
				continue
			}
			log.Println("⚠️ Erro ao puxar mensagens:", err)
			continue
		}

		if len(msgs) == 0 {
			continue
		}

		var wg sync.WaitGroup

		for _, msg := range msgs {
			wg.Add(1)

			go func(m *nats.Msg) {
				defer wg.Done()

				var t Telemetria
				if err := json.Unmarshal(m.Data, &t); err != nil {
					log.Println("❌ Erro no JSON:", err)
					err := m.Ack()
					if err != nil {
						return
					}
					return
				}

				query := `INSERT INTO telemetria (data_hora, imei, latitude, longitude, velocidade) 
				          VALUES ($1, $2, $3, $4, $5)`

				_, err := db.Exec(query, t.Date, t.Imei, t.Latitude, t.Longitude, t.Velocidade)

				if err != nil {
					log.Printf("❌ Erro ao salvar %s: %v\n", t.Imei, err)
					err := m.Nak()
					if err != nil {
						return
					}
				} else {
					err := m.Ack()
					if err != nil {
						return
					}
				}
			}(msg)
		}

		wg.Wait()
		//fmt.Printf("✅ Lote de %d mensagens processado e salvo no banco!\n", len(msgs))
	}
}
