# 🛰️ IoT Data Processing Engine - Guia de Contexto e Arquitetura

Bem-vindo ao **IoT Data Processing Engine**! Este documento (`GEMINI.MD`) funciona como o **manual de contexto central do projeto**. Ele foi projetado especificamente para situar desenvolvedores (humanos ou agentes de Inteligência Artificial) sobre a arquitetura do sistema, fluxo de dados, pilha de tecnologia, configurações de infraestrutura e boas práticas de desenvolvimento.

---

## 🏛️ Arquitetura Geral do Sistema

O sistema é um motor de ingestão de telemetria de alta performance e baixíssima latência para dispositivos IoT (ex: rastreadores de caminhões). Ele é capaz de receber milhões de coordenadas geográficas por segundo, enfileirá-las de forma assíncrona, processar em lotes e persistir em um banco de séries temporais otimizado.

### 🔄 Diagrama de Fluxo de Dados (Data Flow)

O fluxo de processamento de um pacote de telemetria segue a arquitetura orientada a eventos abaixo:

```mermaid
graph TD
    %% Componentes
    subgraph Simulador [Simulador de Dispositivos]
        Gen[telemetry-data-generator<br/>20.000 Goroutines/Trucks]
    end

    subgraph Ingestao [Camada de Ingestão HTTP]
        Ing[telemetry-ingestor<br/>Go HTTP Server :8080]
    end

    subgraph Broker [Fila / Mensageria]
        Nats[NATS JetStream<br/>Subject: iot.telemetria<br/>Stream: STREAM_IOT]
    end

    subgraph Processamento [Camada de Processamento]
        Wrk[telemetry-worker<br/>Pull Subscriber - Lotes de 100]
    end

    subgraph Conexoes [Pool de Conexões]
        Bnc[PgBouncer<br/>Porta :6432 - Session Mode]
    end

    subgraph Armazenamento [Persistência Temporal]
        Db[(TimescaleDB / PostgreSQL)<br/>Hypertable: telemetria]
        Mig[migrator<br/>golang-migrate/v4]
    end

    %% Fluxo
    Gen -->|HTTP POST JSON /telemetria| Ing
    Ing -->|js.Publish| Nats
    Nats -.->|PullSubscribe.Fetch| Wrk
    Wrk -->|Exec INSERTs paralelos| Bnc
    Bnc -->|Proxy SQL| Db
    Mig -->|Aplica DDL inicial| Db
```

---

## 🐳 Detalhamento da Infraestrutura (Docker Compose)

O arquivo [docker-compose.yaml](file:///home/maiconda/iot-data-processing-engine/docker-compose.yaml) gerencia e integra todos os containers do sistema. Abaixo está a explicação detalhada de cada serviço, suas portas e configurações de otimização críticas:

### 1. `timescaledb` (Banco de Dados de Séries Temporais)
* **Imagem**: `timescale/timescaledb:latest-pg16` (PostgreSQL 16 com extensão TimescaleDB embarcada).
* **Porta**: Interna 5432 (protegida de acesso externo direto, exposta apenas via PgBouncer).
* **Parâmetros de Otimização de Performance**:
  - `max_connections=100`: Limita conexões ativas para evitar consumo excessivo de memória no Postgres.
  - `shared_buffers=512MB`: Memória dedicada para cache de dados e índices.
  - `effective_cache_size=1536MB`: Estimativa de memória disponível para cache do sistema operacional.
  - `synchronous_commit=off`: **Altamente Crítico!** Desativa a gravação síncrona obrigatória no disco a cada transação, permitindo escritas assíncronas extremamente rápidas sob volumetria IoT massiva.
  - `wal_buffers=16MB`: Espaço de buffer para o log de transações (Write-Ahead Logging).

### 2. `pgbouncer` (Pool de Conexões SQL)
* **Imagem**: `edoburu/pgbouncer:latest`
* **Porta Exposta**: `6432:5432` (Ponto de entrada seguro e unificado para conexões SQL).
* **Modo de Operação (`POOL_MODE=session`)**: Mantém a conexão física ativa com o PostgreSQL enquanto a sessão do cliente persistir. Ideal para o worker manter canais abertos de inserção concorrente rápida.
* **Limites**:
  - `MAX_CLIENT_CONN=5000`: Permite até 5000 conexões de clientes simultâneas no proxy.
  - `DEFAULT_POOL_SIZE=100`: Mantém até 100 conexões físicas persistentes ativas com o TimescaleDB por trás dos panos.
* **Importância**: O worker de telemetria executa dezenas de conexões concorrentes. Conectar diretamente ao PostgreSQL causaria gargalo ou queda por limite de conexões (`Too Many Connections`). O PgBouncer absorve o pico de clientes e gerencia de forma inteligente a fila de acesso físico ao banco.

### 3. `nats` (Event-Streaming Broker com JetStream)
* **Imagem**: `nats:alpine`
* **Portas Expostas**: `4222:4222` (mensageria/clientes) e `8222:8222` (monitoramento HTTP dashboard).
* **Comando**: `-js -m 8222` (Habilita a engine **JetStream** para persistência física de mensagens em disco e ativa o console de gerência).
* **Importância**: Garante entrega resiliente de mensagens (*At-Least-Once*), mesmo que o processador (worker) passe por lentidão ou reinicializações.

### 4. `migrator` (Inicialização e DDL)
* **Build**: `./migrator`
* **Dependência**: Aguarda o `timescaledb` iniciar.
* **Comportamento**: Roda apenas uma vez (`restart: on-failure`). Aplica as migrations contidas no código Go e encerra sua execução com sucesso, sinalizando para os outros serviços que o banco está pronto para escritas.

### 5. `telemetry-worker` (Processador Assíncrono)
* **Build**: `./telemetry-worker`
* **Dependências**: Aguarda o `nats`, `pgbouncer` e o término bem-sucedido das migrations do `migrator`.
* **Conexão**: Consome mensagens do NATS JetStream e persiste no banco apontando para o `pgbouncer:5432` em vez do banco direto.

### 6. `telemetry-ingestor` (API Gateway de Ingestão)
* **Build**: `./telemetry-ingestor`
* **Porta Exposta**: `8080:8080` (Endpoint exposto para a internet ou rede de trackers).
* **Comportamento**: Recebe os dados de telemetria e os entrega instantaneamente ao NATS.

### 7. `telemetry-data-generator` (Simulador de Tráfego Estressante)
* **Build**: `./telemetry-data-generator`
* **Dependência**: Aguarda o `telemetry-ingestor` estar ativo.
* **Comportamento**: Simula o comportamento real de 20.000 caminhões rodando de forma independente e enviando requisições HTTP POST para o ingestor a cada ~1.5 segundos em média.

---

## 🚀 Guia de Inicialização Rápida

Para subir o ecossistema completo localmente, execute o seguinte comando na raiz do projeto:

```bash
docker compose up --build
```

### Principais Endpoints Úteis
* **API de Ingestão HTTP**: `http://localhost:8080/telemetria` (Apenas POST JSON)
* **Console do NATS**: `http://localhost:8222`
* **Banco de Dados (via PgBouncer)**: `postgres://admin:1234@localhost:6432/iot-database?sslmode=disable`
* **Banco de Dados (Direto)**: `postgres://admin:1234@localhost:5432/iot-database?sslmode=disable` (se exposto manualmente)

---

## 📂 Mapeamento de Módulos (Microserviços)

O projeto é dividido em quatro subdiretórios principais. Cada um possui seu próprio arquivo `GEMINI.MD` detalhando seu funcionamento isolado:

| Módulo | Linguagem | Propósito | Link de Contexto |
| :--- | :---: | :--- | :--- |
| **`migrator`** | Go 1.20+ | Cria a tabela de telemetria e a converte em Hypertable no TimescaleDB. | [migrator/GEMINI.MD](file:///home/maiconda/iot-data-processing-engine/migrator/GEMINI.MD) |
| **`telemetry-data-generator`** | Go 1.20+ | Simulador concorrente de alta escala (20.000 rastreadores IoT simultâneos). | [telemetry-data-generator/GEMINI.MD](file:///home/maiconda/iot-data-processing-engine/telemetry-data-generator/GEMINI.MD) |
| **`telemetry-ingestor`** | Go 1.20+ | API de entrada de baixíssima latência que publica dados no NATS JetStream. | [telemetry-ingestor/GEMINI.MD](file:///home/maiconda/iot-data-processing-engine/telemetry-ingestor/GEMINI.MD) |
| **`telemetry-worker`** | Go 1.20+ | Processador Pull em lotes concorrentes de 100 mensagens para persistência SQL. | [telemetry-worker/GEMINI.MD](file:///home/maiconda/iot-data-processing-engine/telemetry-worker/GEMINI.MD) |

---

## 🤖 Guia de Desenvolvimento para IAs e Desenvolvedores

Ao gerar ou modificar códigos nesta codebase, siga rigorosamente as diretrizes estruturais abaixo:

### 1. Modelo de Contrato de Dados (Telemetria)
Tanto o gerador quanto o ingestor e o worker compartilham a mesma definição exata de payload JSON:
```json
{
  "imei": "TRUCK-0001",
  "lat": -26.5412,
  "lng": -50.1245,
  "velocidade": 85,
  "date": "2026-05-23 17:30:00"
}
```
* **⚠️ Importante**: No banco de dados, o campo `date` (string formatada como `"YYYY-MM-DD HH:MM:SS"`) é mapeado e convertido para a coluna `data_hora` do tipo `TIMESTAMPTZ` (Timestamp com Fuso Horário).

### 2. Otimização de Escrita no Banco de Dados
* Nunca abra ou feche conexões físicas a cada comando INSERT. Use a conexão persistente configurada no Worker via `sql.DB` que por si só faz pool de conexões.
* Sempre insira dados apontando para a porta do **PgBouncer (6432)** em ambientes de produção ou Compose, permitindo maior resiliência concorrente.

### 3. Integração com NATS JetStream
* O ingestor cria automaticamente o Stream `STREAM_IOT` mapeando a máscara de subjects `iot.*`.
* Qualquer novo tipo de telemetria ou evento IoT deve publicar sob tópicos que comecem com `iot.`, ex: `iot.alarmes`, `iot.diagnostico`.
* O processamento no worker é feito via fila distribuída (**Queue Groups**), sob a assinatura `"worker-grupo-1"`, garantindo balanceamento de carga automático caso múltiplos workers estejam rodando em paralelo.
