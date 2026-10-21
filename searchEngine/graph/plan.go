package graph

import (
	"fmt"
	"time"

	"paa/searchEngine/types"
)

// CallPlan é a ordem de chamadas de um resultado da busca: pré-requisitos
// primeiro e o próprio resultado por último.
type CallPlan struct {
	Rank  int // posição do resultado no top-k, a partir de 1
	Steps []*types.Document
}

func (p CallPlan) Prerequisites() []*types.Document { return p.Steps[:len(p.Steps)-1] }

type PlanStats struct {
	WithPrerequisites int // planos com pelo menos um pré-requisito
	Prerequisites     int // soma dos pré-requisitos de todos os planos
	EdgesScanned      int
	Time              time.Duration
}

// Plans roda o Plan para cada resultado, e não só para o primeiro: assim o
// plano da resposta certa existe sempre que ela estiver entre os k.
func (g *Graph) Plans(hits []types.Hit) ([]CallPlan, PlanStats, error) {
	start := time.Now()
	var stats PlanStats
	var plans []CallPlan
	for position, hit := range hits {
		target, ok := g.OperationVertex(hit.Doc.ID)
		if !ok {
			continue
		}
		result := DFS(g, []int{target}, Only(DependsOn))
		stats.EdgesScanned += result.EdgesScanned
		if result.Cycle != nil {
			return nil, stats, fmt.Errorf("ciclo de dependência: %v", result.Cycle)
		}
		plan := CallPlan{Rank: position + 1}
		for _, vertex := range result.Post {
			plan.Steps = append(plan.Steps, g.Vertices[vertex].Doc)
		}
		if prerequisites := len(plan.Prerequisites()); prerequisites > 0 {
			stats.WithPrerequisites++
			stats.Prerequisites += prerequisites
		}
		plans = append(plans, plan)
	}
	stats.Time = time.Since(start)
	return plans, stats, nil
}
