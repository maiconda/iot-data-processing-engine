package main

import (
	"embed"
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var fs embed.FS

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("🚨 ERRO: DATABASE_URL não foi informada")
	}

	fmt.Println("⏳ Iniciando o processo de Migrations...")

	driver, err := iofs.New(fs, "migrations")
	if err != nil {
		log.Fatal("🚨 ERRO ao carregar arquivos de migration: ", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", driver, dbURL)
	if err != nil {
		log.Fatal("🚨 ERRO ao conectar no banco para migração: ", err)
	}

	err = m.Up()

	if err != nil && err != migrate.ErrNoChange {
		log.Fatal("🚨 ERRO FATAL ao aplicar a migration: ", err)
	}

	if err == migrate.ErrNoChange {
		fmt.Println("✅ Nenhuma migration nova para aplicar. Banco de dados já está atualizado.")
	} else {
		fmt.Println("🚀 Migrations aplicadas com sucesso! Hypertable pronta.")
	}
}
