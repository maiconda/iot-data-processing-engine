# 📥 Telemetry Ingestor - Portal de Ingestão de Alta Performance

Este diretório contém o módulo de ingestão HTTP, responsável por expor uma API Gateway pública e receber os payloads de telemetria enviados pelos rastreadores, validando-os e despachando-os instantaneamente para a fila do NATS JetStream.

---

## 🎯 Objetivo e Funcionamento

O **Telemetry Ingestor** é um microserviço **totalmente stateless (sem estado)**. Ele atua como escudo e buffer do sistema:
* **Desacoplamento de Escrita**: Em vez de gravar a telemetria diretamente no banco de dados (o que causaria lentidão e gargalo HTTP), o Ingestor apenas publica o evento no NATS JetStream em microssegundos e libera a conexão do cliente.
* **Escalabilidade Horizontal**: Por não guardar estado em memória local, múltiplos containers do Ingestor podem ser colocados atrás de um Balanceador de Carga (Load Balancer) para escalar infinitamente conforme o tamanho da frota IoT cresce.

---

## 🛣️ Rota HTTP `/telemetria` (POST)

O serviço expõe um servidor HTTP padrão Go escutando na porta `PORT` (padrão `8080`).

### Validação de Entrada
* **Método**: Apenas requisições `POST` são permitidas. Qualquer outro método retorna `405 Method Not Allowed`.
* **Payload JSON**: O corpo da requisição é mapeado para a struct `Telemetria` em Go. Se o JSON estiver quebrado, corrompido ou fora de especificação, a API responde imediatamente com `400 Bad Request`, evitando poluir a fila de mensageria com lixo.

---

## 🛡️ Integração Automática com NATS JetStream

Na inicialização do microserviço (`main.go`), o Ingestor executa a rotina de auto-configuração do broker NATS:

1. **Conexão ao Broker**: Conecta através da variável `NATS_URL`.
2. **Setup do Stream (`STREAM_IOT`)**: 
   - Consulta o NATS para verificar se o Stream `STREAM_IOT` já foi criado.
   - Caso não exista, o Ingestor declara e cria o stream dinamicamente sob as seguintes configurações:
     ```go
     _, errStream = js.AddStream(&nats.StreamConfig{
         Name:     "STREAM_IOT",
         Subjects: []string{"iot.*"},
     })
     ```
   - **Importância**: O uso da máscara `iot.*` permite que este mesmo stream gerencie e persista múltiplos tópicos de dados IoT no futuro (ex: `iot.telemetria`, `iot.alarmes`, `iot.diagnostico`) sem precisar mudar a infraestrutura.

3. **Publicação Assíncrona**:
   - Cada telemetria válida recebida na rota HTTP é convertida em JSON binário e publicada no subject exato `iot.telemetria` através da chamada `js.Publish("iot.telemetria", payload)`.

---

## ⚙️ Variáveis de Ambiente Necessárias

| Variável | Descrição | Valor Padrão |
| :--- | :--- | :--- |
| **`PORT`** | Porta de escuta do servidor HTTP local. | `8080` |
| **`NATS_URL`** | Endereço de rede para conexão com o broker NATS. | `nats://localhost:4222` |

---

## 🤖 Manual do Desenvolvedor: Expandindo a API

### Como adicionar um novo endpoint de telemetria?
Se você precisar criar uma rota para receber dados de alarmes dos veículos (ex: Botão de Pânico acionado):
1. No arquivo [main.go](file:///home/maiconda/iot-data-processing-engine/telemetry-ingestor/main.go), crie uma nova struct, ex: `Alarme`.
2. Crie uma nova função handler, ex: `func receberAlarme(w http.ResponseWriter, r *http.Request)`.
3. Valide o método e o corpo do JSON.
4. Publique a mensagem em um subject correspondente do NATS, ex: `js.Publish("iot.alarmes", payloadJSON)`.
5. Associe a rota no `main()`:
   ```go
   http.HandleFunc("/alarmes", receberAlarme)
   ```
6. Como o Stream `STREAM_IOT` foi configurado com o wildcard `iot.*`, o NATS irá armazenar automaticamente as mensagens do novo tópico sem precisar reconfigurar o broker!
