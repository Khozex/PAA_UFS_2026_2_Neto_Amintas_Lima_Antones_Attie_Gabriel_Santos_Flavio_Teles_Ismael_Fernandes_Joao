// Servidor web da interface: serve a app React (ui/web/dist) e expõe a ponte
// Ollama ↔ MCP por HTTP.
//
//	GET  /api/models          modelos instalados no Ollama e se aceitam tools
//	GET  /api/info            tools do servidor MCP
//	POST /api/chat            {model, messages:[{role,content}], search:{k,config,select,order_strategy}}
//	                          → eventos SSE do bridge
//
//	go run ./ui/server [-addr :8080] [-static ui/web/dist] [-- flags do ui/mcp]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"paa/searchEngine/utils"
	"paa/ui/bridge"
)

type chatBody struct {
	Model    string               `json:"model"`
	Messages []bridge.Message     `json:"messages"`
	Search   bridge.SearchOptions `json:"search"`
}

func main() {
	addr := flag.String("addr", ":8080", "endereço HTTP")
	static := flag.String("static", "ui/web/dist", "pasta com o build da app React (npm run build em ui/web)")
	url := flag.String("ollama", "http://localhost:11434", "endereço do Ollama")
	numCtx := flag.Int("num_ctx", 8192, "janela de contexto do modelo")
	k := flag.Int("k", 5, "quantas operações recuperar por pergunta")
	mcpBin := flag.String("mcp", "", "binário do servidor MCP; vazio = go run ./ui/mcp")
	flag.Parse()

	ctx := context.Background()
	b, err := bridge.Connect(ctx, *url, *numCtx, *mcpBin, flag.Args())
	if err != nil {
		log.Fatal(err)
	}
	defer b.Close()
	b.K = *k

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/models", func(w http.ResponseWriter, r *http.Request) {
		models, err := b.Models(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, models)
	})
	mux.HandleFunc("GET /api/info", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"tools": b.ToolNames, "ollama": b.OllamaURL})
	})
	mux.HandleFunc("POST /api/chat", func(w http.ResponseWriter, r *http.Request) {
		var body chatBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Model == "" || len(body.Messages) == 0 {
			http.Error(w, "esperado {model, messages:[{role,content}]}", http.StatusBadRequest)
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming não suportado", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")
		emit := func(e bridge.Event) {
			raw, _ := json.Marshal(e)
			fmt.Fprintf(w, "data: %s\n\n", raw)
			flusher.Flush()
		}
		if _, err := b.Ask(r.Context(), body.Model, body.Messages, body.Search, emit); err != nil {
			log.Printf("chat: %v", err)
		}
	})

	dist := utils.Resolve(*static)
	if _, err := os.Stat(filepath.Join(dist, "index.html")); err != nil {
		log.Printf("aviso: %s não tem index.html; rode `npm run build` em ui/web ou use `npm run dev` com proxy", dist)
	}
	mux.Handle("/", spa(dist))

	log.Printf("ui em http://localhost%s  ollama=%s  tools=%s", *addr, b.OllamaURL, strings.Join(b.ToolNames, ","))
	log.Fatal(http.ListenAndServe(*addr, mux))
}

// spa serve os arquivos do build e cai no index.html para qualquer outra rota.
func spa(dist string) http.Handler {
	files := http.FileServer(http.Dir(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dist, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			files.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(dist, "index.html"))
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
