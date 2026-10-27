# Cotação USD-BRL — client e server em Go

Dois programas que trocam a cotação do dólar respeitando timeouts estritos com `context`.

| Programa | O que faz | Timeout |
|---|---|---|
| `server` | Escuta em `:8080`, expõe `GET /cotacao`, consulta a [AwesomeAPI](https://economia.awesomeapi.com.br/json/last/USD-BRL), grava a cotação em SQLite (`cotacoes.db`) e devolve `{"bid": "..."}` | API externa: **200ms** · Banco: **10ms** |
| `client` | Chama `http://localhost:8080/cotacao`, lê o `bid` e grava `cotacao.txt` com `Dólar: {valor}` | **300ms** |

Estouros de timeout são logados no console de quem os sofreu (server ou client).

## Requisitos

- Go 1.27 (versão declarada no `go.mod`)
- Não precisa de CGO nem de compilador C: o SQLite usa o driver [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) (Go puro)

## Como rodar

```bash
go mod download
```

Terminal 1 — servidor:

```bash
go run ./server
```

Terminal 2 — cliente (com o servidor no ar):

```bash
go run ./client
```

O cliente cria `cotacao.txt` na pasta onde foi executado:

```
Dólar: 5.4321
```

O servidor cria `cotacoes.db` na pasta onde foi executado; para conferir os registros:

```bash
sqlite3 cotacoes.db "SELECT * FROM cotacoes;"
```

## Testes

```bash
go vet ./...
go test ./...
```

## Observações

- Se o INSERT estourar os 10ms, o erro é logado no servidor e o cliente ainda recebe a cotação (o registro daquela chamada não é gravado).
- Se a API externa estourar os 200ms, o servidor loga o erro e responde `504`; o cliente então loga o erro e não grava o arquivo.
- O banco usa WAL e `synchronous=NORMAL` para o INSERT caber com folga nos 10ms.
