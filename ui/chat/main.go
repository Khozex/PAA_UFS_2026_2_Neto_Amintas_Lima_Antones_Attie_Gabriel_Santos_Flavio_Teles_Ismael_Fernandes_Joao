// Chat de terminal sobre o bridge Ollama ↔ MCP. Serve para testar a ponte sem
// a interface web (ui/server + ui/web).
//
//	go run ./ui/chat                              # REPL
//	go run ./ui/chat -once "how do I cancel a workflow run?"
//	go run ./ui/chat -model qwen2.5:3b -- -preset 3 -k 5   # flags após -- vão para ui/mcp
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"paa/ui/bridge"
)

func main() {
	model := flag.String("model", "qwen2.5:3b", "modelo do Ollama com suporte a tools")
	url := flag.String("ollama", "http://localhost:11434", "endereço do Ollama")
	numCtx := flag.Int("num_ctx", 8192, "janela de contexto do modelo")
	mcpBin := flag.String("mcp", "", "binário do servidor MCP; vazio = go run ./ui/mcp")
	once := flag.String("once", "", "faz uma pergunta só e sai")
	verbose := flag.Bool("v", false, "mostra rodadas, tempo e tokens de cada pergunta")
	k := flag.Int("k", 5, "quantas operações recuperar por pergunta")
	flag.Parse()

	ctx := context.Background()
	b, err := bridge.Connect(ctx, *url, *numCtx, *mcpBin, flag.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer b.Close()
	b.K = *k

	var history []bridge.Message
	ask := func(text string) error {
		history = append(history, bridge.Message{Role: "user", Content: text})
		answer, err := b.Ask(ctx, *model, history, bridge.SearchOptions{}, func(e bridge.Event) {
			switch e.Type {
			case "tool_call":
				args, _ := json.Marshal(e.Args)
				fmt.Fprintf(os.Stderr, "  🔎 %s %s\n", e.Name, args)
			case "done":
				if *verbose {
					fmt.Fprintf(os.Stderr, "  [rodadas=%d tools=%d prompt=%d tokens out=%d tokens llm=%s]\n",
						e.Stats.Rounds, e.Stats.ToolCalls, e.Stats.PromptTokens, e.Stats.OutputTokens, e.Stats.LLMTime.Round(1e6))
				}
			}
		})
		if err != nil {
			return err
		}
		history = append(history, bridge.Message{Role: "assistant", Content: answer})
		fmt.Printf("\n%s> %s\n", *model, answer)
		return nil
	}

	if *once != "" {
		if err := ask(*once); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	fmt.Fprintf(os.Stderr, "modelo=%s  tools=%s  (Ctrl-D para sair)\n", *model, strings.Join(b.ToolNames, ","))
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\nvocê> ")
		if !scanner.Scan() {
			fmt.Println()
			return
		}
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		if err := ask(text); err != nil {
			fmt.Fprintln(os.Stderr, "erro:", err)
		}
	}
}
