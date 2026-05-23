# ⚙️ Telemetry Worker - Processador e Consumidor de Lotes Concorrente

Este diretório contém o módulo processador/worker, responsável por consumir assincronamente as mensagens de telemetria enfileiradas no NATS JetStream, processá-las em lotes paralelos de alta performance e persisti-las com segurança no banco de dados TimescaleDB.

---

## 🎯 Objetivo e Funcionamento

O **Telemetry Worker** é o coração operacional de persistência do sistema. Ele atua como um consumidor do tipo *Pull* (Puxada), o que significa que ele controla a taxa de consumo de dados de acordo com a sua própria capacidade de processamento, evitando sobrecarregar o banco de dados.

---

## 🏎️ Modelo Pull Subscriber e Divisão de Carga (Queue Groups)

O worker utiliza a assinatura do tipo Pull do JetStream:

```go
sub, err := js.PullSubscribe("iot.telemetria", "worker-grupo-1")
```

### Principais Características da Arquitetura:
1. **Balanceamento Automático (Queue Group)**: O identificador `"worker-grupo-1"` define um **Grupo de Fila** no NATS. Se você subir 10 instâncias deste Worker no Docker ou Kubernetes, o NATS distribuirá as mensagens de forma balanceada e automática entre todas as instâncias ativas, garantindo **alta disponibilidade** e **processamento distribuído** sem risco de duplicidade.
2. **Consumo em Lote (Batching)**: O loop principal do worker solicita mensagens em blocos de até 100 itens (`tamanhoLote = 100`) a cada ciclo de leitura com timeout de 2 segundos:
   ```go
   msgs, err := sub.Fetch(100, nats.MaxWait(2*time.Second))
   ```
   Isso reduz drasticamente a latência de rede e a sobrecarga de round-trip de comunicação com o broker se comparado ao consumo unitário (mensagem por mensagem).

---

## ⚡ Concorrência Massiva com `sync.WaitGroup`

Uma vez que o lote de 100 mensagens é coletado, o Worker processa cada uma das mensagens de forma **assíncrona e concorrente** usando Goroutines locais e sincronização via `sync.WaitGroup`:

```go
var wg sync.WaitGroup

for _, msg := range msgs {
    wg.Add(1)
    go func(m *nats.Msg) {
        defer wg.Done()
        // 1. Unmarshal do JSON
        // 2. Executa INSERT no banco
        // 3. Ack ou Nak na mensagem
    }(msg)
}

wg.Wait() // Aguarda todas as 100 goroutines do lote terminarem
```

### Por que usar essa estratégia?
* **Paralelização de I/O**: Em vez de fazer 100 inserts síncronos sequenciais (o que demoraria muito por conta da latência de rede com o banco), o worker dispara os 100 inserts simultaneamente.
* **Sincronização Segura**: O `wg.Wait()` garante que o worker só solicitará o próximo lote de 100 mensagens do NATS depois que **todas** as inserções do lote atual forem resolvidas, atuando como um mecanismo natural de controle de fluxo (*backpressure*).

---

## 🛡️ Conexões com PgBouncer e Robustez Transacional (`Ack`/`Nak`)

### Integração com PgBouncer
O driver SQL em Go está configurado para otimizar o uso do pool de conexões gerenciado pelo PgBouncer:
* `db.SetMaxOpenConns(100)`: Limita o número de conexões SQL simultâneas que o Go abrirá com o PgBouncer.
* `db.SetMaxIdleConns(10)`: Mantém conexões inativas quentes no pool interno de Go para reaproveitamento rápido.

### Garantia de Entrega (Resiliência de Dados)
A confirmação de recebimento da mensagem no NATS segue a lógica de segurança:
* **`Ack()` (Acknowledgment)**: Enviado ao NATS após o sucesso do `INSERT` no banco. A mensagem é removida definitivamente da fila.
* **`Nak()` (Negative Acknowledgment)**: Enviado caso ocorra falha de inserção no banco (ex: banco temporariamente offline ou sobrecarregado). O NATS devolve a mensagem à fila instantaneamente para reprocessamento futuro.
* **Tratamento de Dados Inválidos**: Se a mensagem vier com JSON corrompido, o worker loga o erro e envia `Ack()` para descartar a mensagem, evitando que um dado malformado trave a fila em um loop infinito de erros.

---

## ⚙️ Variáveis de Ambiente Necessárias

| Variável | Descrição | Exemplo |
| :--- | :--- | :--- |
| **`DATABASE_URL`** | String de conexão direcionada para a porta do PgBouncer. | `postgres://admin:1234@pgbouncer:5432/iot-database?sslmode=disable` |
| **`NATS_URL`** | Endereço de rede para conexão com o broker NATS. | `nats://nats:4222` |

---

## 🤖 Manual do Desenvolvedor: Ajustando Ajustes de Performance

* **Alteração do Tamanho do Lote**: Para cenários de volumes maiores, você pode alterar `tamanhoLote` (ex: de `100` para `500` ou `1000`). Certifique-se de validar se o limite de conexões do PgBouncer e TimescaleDB suporta o novo volume.
* **Ajuste de Conexões**: Se alterar o tamanho do lote para `500`, aumente proporcionalmente o `db.SetMaxOpenConns(500)` para que o Go não bloqueie as goroutines aguardando slots de conexões livres.
