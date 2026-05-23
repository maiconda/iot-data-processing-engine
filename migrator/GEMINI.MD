# 🗄️ Migrator - Módulo de Inicialização e Schema Banco de Dados

Este diretório contém o módulo de migrações automáticas do banco de dados, responsável por garantir a integridade estrutural e a preparação inicial do TimescaleDB antes que os microserviços de ingestão e processamento de telemetria entrem em operação.

---

## 🎯 Objetivo e Funcionamento

O **Migrator** é executado como uma tarefa única de curta duração (*Job*). Ele conecta-se diretamente ao banco de dados Postgres (`timescaledb`), carrega os arquivos SQL embutidos na compilação do binário Go e aplica as migrações necessárias.

O fluxo de dependências é coordenado no Docker Compose:
1. O banco `timescaledb` inicia.
2. O `migrator` roda as migrations.
3. Apenas após o sucesso do `migrator`, os serviços `pgbouncer` e `telemetry-worker` são liberados para conexão e processamento.

---

## 🏛️ Modelagem e Schema: Tabela `telemetria`

A migration inicial ([000001_migration.up.sql](file:///home/maiconda/iot-data-processing-engine/migrator/migrations/000001_migration.up.sql)) cria a tabela de séries temporais estruturada para altíssimo volume de escrita.

### DDL Aplicada
```sql
CREATE TABLE IF NOT EXISTS telemetria
(
    data_hora TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    imei TEXT NOT NULL,
    latitude DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,
    velocidade INTEGER NOT NULL
);

SELECT create_hypertable('telemetria', 'data_hora');

CREATE INDEX idx_telemetria_imei_tempo ON telemetria (imei, data_hora DESC);
```

### Explicação Detalhada das Colunas
* **`data_hora` (`TIMESTAMPTZ`)**: Armazena o registro exato de data e hora do evento de telemetria com fuso horário. É a **coluna de partição primária**. O uso de `TIMESTAMPTZ` é crítico para evitar distorções de fuso horário em frotas globais de dispositivos.
* **`imei` (`TEXT`)**: Identificação única internacional do modem do rastreador do veículo (ex: `TRUCK-0001`). Usado para agrupar e pesquisar o histórico de um dispositivo específico.
* **`latitude` e `longitude` (`DOUBLE PRECISION`)**: Coordenadas geográficas de ponto flutuante de alta precisão de 64 bits, essenciais para plotagem precisa em mapas.
* **`velocidade` (`INTEGER`)**: Velocidade atual do veículo registrada pelo sensor do rastreador (em km/h).

---

## ⚡ TimescaleDB Hypertables (Tabelas Hiperdimensionais)

A instrução `SELECT create_hypertable('telemetria', 'data_hora');` converte uma tabela comum do PostgreSQL em uma **Hypertable** do TimescaleDB.

### Por que usar Hypertables?
1. **Particionamento Automático**: O TimescaleDB divide a tabela fisicamente em "chunks" (pedaços) menores com base no intervalo de tempo da coluna `data_hora`.
2. **Alta Performance de Gravação**: Ao fazer inserções em massa, o banco grava sempre no chunk ativo correspondente ao tempo atual, mantendo os índices leves e cabendo na memória RAM. Isso evita a degradação clássica de performance que tabelas relacionais normais sofrem ao passar de dezenas de milhões de linhas.
3. **Descarte Rápido de Dados antigos (Data Retention)**: Permite limpar dados históricos deletando chunks inteiros instantaneamente, sem gerar bloqueios de tabela ou consumo de CPU pesado de comandos `DELETE` tradicionais.

---

## 🔍 Índice Composto Otimizado

```sql
CREATE INDEX idx_telemetria_imei_tempo ON telemetria (imei, data_hora DESC);
```
O índice criado é composto por `(imei, data_hora DESC)`. Ele foi projetado sob medida para consultas clássicas de rastreamento:
* **Caso de Uso**: *"Onde está o caminhão X agora?"* ou *"Mostre o trajeto do caminhão X nas últimas 2 horas"*.
* **Por que DESC?**: O fuso decrescente (`DESC`) permite que o banco localize instantaneamente os últimos registros geográficos mais recentes do dispositivo, economizando precioso tempo de processamento de busca.

---

## 🛠️ Detalhes de Implementação em Go

O migrador é escrito em Go e faz uso de duas tecnologias fundamentais para robustez de entrega:

### 1. Sistema de Arquivos Embarcado (`embed.FS`)
O código Go utiliza a diretiva `//go:embed migrations/*.sql` para compilar fisicamente todos os arquivos `.sql` para dentro do executável compilado final.
* **Benefício**: Elimina a necessidade de mapear volumes do Docker ou copiar arquivos de migration adicionais para dentro do container em produção. O executável é 100% autossuficiente.

### 2. Biblioteca `golang-migrate`
Gerencia a aplicação e o versionamento do banco através da biblioteca robusta `github.com/golang-migrate/migrate/v4`. Ela cria automaticamente uma tabela interna de controle (`schema_migrations`) para garantir que nenhuma migration seja aplicada duas vezes ou em ordem errada.

---

## ⚙️ Variáveis de Ambiente Necessárias

| Variável | Descrição | Exemplo |
| :--- | :--- | :--- |
| **`DATABASE_URL`** | String de conexão com o TimescaleDB contendo credenciais, host e porta. | `postgres://admin:1234@timescaledb:5432/iot-database?sslmode=disable` |

---

## 🤖 Manual do Desenvolvedor: Criando Novas Migrations

Caso precise alterar o schema do banco (ex: adicionar uma tabela de alertas de velocidade):

1. Crie um novo arquivo de migration na pasta `migrator/migrations/` seguindo estritamente a nomenclatura incremental:
   - `000002_add_alerts_table.up.sql` (contendo os comandos `CREATE TABLE` / `ALTER TABLE`).
   - `000002_add_alerts_table.down.sql` (contendo os comandos de rollback, como `DROP TABLE`).
2. O sistema de arquivos embarcado (`embed.FS`) lerá a nova migration automaticamente na próxima execução.
3. Certifique-se de que os scripts SQL sejam **idempotentes** (use `CREATE TABLE IF NOT EXISTS`, `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`, etc.).
