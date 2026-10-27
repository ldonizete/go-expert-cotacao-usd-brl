package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	serverURL = "http://localhost:8080/cotacao"
	timeout   = 300 * time.Millisecond
	outFile   = "cotacao.txt"
)

type bidResponse struct {
	Bid string `json:"bid"`
}

func main() {
	bid, err := fetchBid(serverURL, timeout)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			log.Fatalf("timeout de %s excedido ao consultar o servidor: %v", timeout, err)
		}
		log.Fatalf("erro ao consultar o servidor: %v", err)
	}

	if err := os.WriteFile(outFile, []byte(fmt.Sprintf("Dólar: %s", bid)), 0o644); err != nil {
		log.Fatalf("erro ao gravar %s: %v", outFile, err)
	}
	log.Printf("cotação gravada em %s: Dólar: %s", outFile, bid)
}

func fetchBid(url string, limit time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("servidor retornou status %d", resp.StatusCode)
	}

	var data bidResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}
	if data.Bid == "" {
		return "", errors.New("resposta sem o campo bid")
	}
	return data.Bid, nil
}
