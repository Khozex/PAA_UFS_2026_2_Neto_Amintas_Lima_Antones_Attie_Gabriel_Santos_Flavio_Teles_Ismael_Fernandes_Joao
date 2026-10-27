package main

import (
	"slices"
	"testing"

	"paa/searchEngine/types"
)

var testDocs = []types.Document{
	{ID: "repos/list-releases", Method: "get", Path: "/repos/{owner}/{repo}/releases"},
	{ID: "repos/delete-release", Method: "delete", Path: "/repos/{owner}/{repo}/releases/{release_id}"},
	{ID: "repos/get-release", Method: "get", Path: "/repos/{owner}/{repo}/releases/{release_id}"},
	{ID: "repos/rename-branch", Method: "post", Path: "/repos/{owner}/{repo}/branches/{branch}/rename"},
	{ID: "users/update-authenticated", Method: "patch", Path: "/user"},
}

func TestCallsIn(t *testing.T) {
	answer := "Use `DELETE /repos/{owner}/{repo}/releases/{release_id}`.\n" +
		"curl -X POST https://api.github.com/repos/<OWNER>/<REPO>/branches/<BRANCH>/rename\n" +
		"curl \"https://api.github.com/repos/o/r/releases?per_page=5\" -X GET\n" +
		"De novo: DELETE /repos/{owner}/{repo}/releases/{release_id}."
	want := []call{
		{"DELETE", "/repos/{owner}/{repo}/releases/{release_id}"},
		{"POST", "/repos/<OWNER>/<REPO>/branches/<BRANCH>/rename"},
		{"GET", "/repos/o/r/releases"},
	}
	if got := callsIn(answer); !slices.Equal(got, want) {
		t.Errorf("chamadas %v, esperava %v", got, want)
	}
}

func TestMatchesTemplate(t *testing.T) {
	template := "/repos/{owner}/{repo}/releases"
	cases := []struct {
		path string
		want bool
	}{
		{"/repos/{owner}/{repo}/releases", true},
		{"/repos/octocat/hello/releases", true},
		{"/repos/{owner}/{repo}/releases/{release_id}", false},
		{"/repos/{owner}/{repo}/tags", false},
	}
	for _, c := range cases {
		if got := matchesTemplate(template, c.path); got != c.want {
			t.Errorf("%q: %v, esperava %v", c.path, got, c.want)
		}
	}
}

func TestScore(t *testing.T) {
	deleteRelease := func(answer string) record {
		r := record{
			Relevant:      []string{"repos/delete-release"},
			Prerequisites: [][]string{{"repos/list-releases"}},
			ContextIDs:    []string{"repos/delete-release"},
			Answer:        answer,
		}
		score(&r, testDocs)
		return r
	}

	r := deleteRelease("Primeiro GET /repos/{owner}/{repo}/releases, depois DELETE /repos/{owner}/{repo}/releases/{release_id}.")
	if !r.Correct || !r.CitedPrereq || len(r.Invented) > 0 || r.ContextUse != "trouxe_e_usou" {
		t.Errorf("resposta completa: %+v", r)
	}

	r = deleteRelease("curl -X DELETE https://api.github.com/repos/o/r/releases/42")
	if !r.Correct || r.CitedPrereq {
		t.Errorf("acerto só pelo curl, sem pré-requisito: %+v", r)
	}

	r = deleteRelease("Use PUT /repos/{owner}/{repo}/releases/{release_id} e depois DELETE /user.")
	if r.Correct || len(r.Invented) != 2 || r.ContextUse != "trouxe_e_nao_usou" {
		t.Errorf("método errado e endpoint inexistente contam como inventados: %+v", r)
	}

	r = deleteRelease("GET /repos/{owner}/{repo}/releases")
	if r.Correct || r.CitedPrereq {
		t.Errorf("só a listagem não acerta a operação nem conta o pré-requisito: %+v", r)
	}

	outside := record{Answer: "Não existe operação para trocar a foto de perfil."}
	score(&outside, testDocs)
	if !outside.Correct || outside.ContextUse != "" {
		t.Errorf("fora do corpus sem chamada: %+v", outside)
	}
	outside = record{Answer: "Não existe, mas PATCH /user muda o perfil."}
	score(&outside, testDocs)
	if outside.Correct {
		t.Errorf("fora do corpus citando chamada: %+v", outside)
	}
}

func TestScoreNeedsEveryPrerequisiteStep(t *testing.T) {
	r := record{
		Relevant:      []string{"repos/delete-release"},
		Prerequisites: [][]string{{"repos/list-releases"}, {"repos/get-release"}},
		Answer:        "GET /repos/{owner}/{repo}/releases e DELETE /repos/{owner}/{repo}/releases/{release_id}",
	}
	score(&r, testDocs)
	if r.CitedPrereq {
		t.Errorf("faltou o segundo passo e contou como citado: %+v", r)
	}
	if r.ContextUse != "de_memoria" {
		t.Errorf("acertou sem a operação no contexto: %q", r.ContextUse)
	}
}
