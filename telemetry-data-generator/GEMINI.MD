# 🚜 Telemetry Data Generator - Simulador IoT Concorrente

Este diretório contém o módulo gerador/simulador de dados de telemetria, responsável por simular uma frota massiva de rastreadores veiculares e enviar dados de geolocalização e velocidade de forma ininterrupta para a API de Ingestão.

---

## 🎯 Objetivo e Funcionamento

O **Telemetry Data Generator** foi projetado para atuar como uma **ferramenta de estresse e teste de carga**. Ele garante que o motor de ingestão HTTP e o broker de mensagens NATS JetStream funcionem corretamente sob cenários de uso extremos do mundo real, simulando milhares de requisições paralelas por segundo.

---

## 🚀 Concorrência em Altíssima Escala (Goroutines)

O simulador é escrito em Go e faz uso extensivo do modelo de concorrência nativo da linguagem (**Goroutines**). 

No método `main()`, o simulador dispara um loop de **20.000 iterações**:

```go
func main() {
	for i := 1; i <= 20000; i++ {
		imei := fmt.Sprintf("TRUCK-%04d", i)
		go simularRastreador(imei)
	}
	select {}
}
```

### Por que esta abordagem é extremamente eficiente?
* **Threads Leves**: Goroutines são threads lógicas gerenciadas pelo runtime do Go (e não pelo Kernel do SO). Cada goroutine consome apenas cerca de 2 KB a 4 KB de memória inicial.
* **Escala Massiva**: Spawnar 20.000 threads de SO tradicionais travaria o servidor por falta de memória e overhead de troca de contexto. Com as 20.000 goroutines do Go, o agendador nativo (`Go Scheduler`) gerencia tudo de forma ultraeficiente com pouquíssimo uso de CPU e memória RAM.

---

## 📐 Algoritmo de Simulação de Dados

Cada goroutine de rastreador (`simularRastreador`) executa em um loop infinito as seguintes etapas:

1. **Geração de Coordenadas Geográficas Razoáveis**:
   - As coordenadas geradas simulam deslocamentos reais flutuando matematicamente ao redor do sul do Brasil (próximo a Santa Catarina/Paraná):
   - **Latitude**: `-27.0 + (rand.Float64() * 2)` (Varia entre `-27.0` e `-25.0`)
   - **Longitude**: `-51.0 + (rand.Float64() * 2)` (Varia entre `-51.0` e `-49.0`)
2. **Geração de Velocidade**:
   - **Velocidade**: `rand.Intn(120)` (Inteiro aleatório entre `0` e `119` km/h).
3. **Estampa de Tempo (Timestamp)**:
   - Formata a hora atual do sistema em formato textual padrão brasileiro SQL: `"YYYY-MM-DD HH:MM:SS"`.
4. **Intervalo Flutuante (Jitter)**:
   - Após enviar a telemetria, o rastreador aguarda um tempo aleatório de 0 a 3 segundos (`rand.Intn(3000) * time.Millisecond`).
   - **Importância**: O *Jitter* (variação do atraso) impede o efeito de *Stampeding Herd* (efeito manada), onde todos os 20.000 rastreadores disparariam requisições no mesmo milissegundo exato, distribuindo a carga de forma uniforme ao longo do tempo.

---

## 🔒 Robustez HTTP e Prevenção de Memory Leaks

O simulador implementa boas práticas de rede no consumo de conexões HTTP:

```go
resposta, erroHTTP := http.Post(urlDestino, "application/json", bytes.NewBuffer(payloadJSON))
if erroHTTP != nil {
    fmt.Printf("🔴 [Rastreador %s] Falha de conexão: Servidor offline!\n", imei)
} else {
    err := resposta.Body.Close()
    if err != nil {
        return
    }
}
```
* **Resiliência a Falhas**: Se o `telemetry-ingestor` cair ou estiver sobrecarregado, o gerador não trava. Ele apenas loga `"Servidor offline!"` e aguarda o próximo intervalo de sono para tentar novamente.
* **Fechamento de Body**: Sempre que a requisição HTTP é bem-sucedida, o `resposta.Body.Close()` é chamado imediatamente. Isso evita vazamento de sockets e conexões TCP órfãs no sistema operacional (*Socket Leak*), mantendo a estabilidade a longo prazo.

---

## ⚙️ Variáveis de Ambiente Necessárias

| Variável | Descrição | Valor Padrão |
| :--- | :--- | :--- |
| **`URL_INGESTOR`** | Endpoint HTTP completo para onde os dados serão enviados via POST. | `http://localhost:8080/telemetria` |

---

## 🤖 Manual do Desenvolvedor: Ajustando a Escala de Testes

Caso precise alterar o volume de dados de teste:
1. Abra o arquivo [main.go](file:///home/maiconda/iot-data-processing-engine/telemetry-data-generator/main.go).
2. Altere o limite do loop no método `main()` (ex: altere `20000` para `1000` se quiser rodar um teste mais leve localmente em computadores com menos recursos).
3. Para ajustar o intervalo entre envios, mude o parâmetro do gerador de milissegundos: `rand.Intn(5000)` para envios a cada 5 segundos, por exemplo.
