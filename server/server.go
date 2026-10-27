package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	_ "modernc.org/sqlite"
)

const (
	addr       = ":8080"
	apiURL     = "https://economia.awesomeapi.com.br/json/last/USD-BRL"
	apiTimeout = 200 * time.Millisecond
	dbTimeout  = 10 * time.Millisecond
	// WAL + synchronous=NORMAL mantém o INSERT dentro dos 10ms na maioria dos casos.
	dbDSN = "file:cotacoes.db?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)"
)

type apiResponse struct {
	USDBRL cotacao `json:"USDBRL"`
}

type cotacao struct {
	Code       string `json:"code"`
	Codein     string `json:"codein"`
	Name       string `json:"name"`
	High       string `json:"high"`
	Low        string `json:"low"`
	VarBid     string `json:"varBid"`
	PctChange  string `json:"pctChange"`
	Bid        string `json:"bid"`
	Ask        string `json:"ask"`
	Timestamp  string `json:"timestamp"`
	CreateDate string `json:"create_date"`
}

type bidResponse struct {
	Bid string `json:"bid"`
}

type server struct {
	db         *sql.DB
	apiURL     string
	apiTimeout time.Duration
	dbTimeout  time.Duration
}

func main() {
	db, err := sql.Open("sqlite", dbDSN)
	if err != nil {
		log.Fatalf("abrindo banco: %v", err)
	}
	defer db.Close()

	if err := initDB(db); err != nil {
		log.Fatalf("criando tabela: %v", err)
	}

	s := &server{db: db, apiURL: apiURL, apiTimeout: apiTimeout, dbTimeout: dbTimeout}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /cotacao", s.handleCotacao)

	log.Printf("servidor ouvindo em %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

// initDB roda na inicialização, fora do timeout de 10ms do INSERT.
func initDB(db *sql.DB) error {
	db.SetMaxOpenConns(1)
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS cotacoes (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		bid        TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	return err
}

func (s *server) handleCotacao(w http.ResponseWriter, r *http.Request) {
	c, err := s.fetchCotacao(r.Context())
	if err != nil {
		logErr("consultar API de câmbio", err, s.apiTimeout)
		status := http.StatusBadGateway
		if errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusGatewayTimeout
		}
		http.Error(w, "erro ao consultar a cotação", status)
		return
	}

	// Falha ao persistir é logada, mas o cliente ainda recebe a cotação.
	if err := s.saveCotacao(r.Context(), c.Bid); err != nil {
		logErr("gravar cotação no banco", err, s.dbTimeout)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(bidResponse{Bid: c.Bid}); err != nil {
		log.Printf("erro ao escrever resposta: %v", err)
	}
}

func (s *server) fetchCotacao(parent context.Context) (*cotacao, error) {
	ctx, cancel := context.WithTimeout(parent, s.apiTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API retornou status %d", resp.StatusCode)
	}

	var data apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	if data.USDBRL.Bid == "" {
		return nil, errors.New("resposta da API sem o campo bid")
	}
	return &data.USDBRL, nil
}

func (s *server) saveCotacao(parent context.Context, bid string) error {
	ctx, cancel := context.WithTimeout(parent, s.dbTimeout)
	defer cancel()

	_, err := s.db.ExecContext(ctx, `INSERT INTO cotacoes (bid) VALUES (?)`, bid)
	return err
}

func logErr(op string, err error, limit time.Duration) {
	if errors.Is(err, context.DeadlineExceeded) {
		log.Printf("timeout de %s excedido ao %s: %v", limit, op, err)
		return
	}
	log.Printf("erro ao %s: %v", op, err)
}
