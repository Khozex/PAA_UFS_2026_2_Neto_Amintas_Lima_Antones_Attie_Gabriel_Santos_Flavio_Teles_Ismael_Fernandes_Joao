package main

import "testing"

func TestMentionsCall(t *testing.T) {
	list := "/repos/{owner}/{repo}/releases"
	cases := []struct {
		answer string
		want   bool
	}{
		{"Use `DELETE /repos/{owner}/{repo}/releases/{release_id}`.", false},
		{"GET /repos/{owner}/{repo}/releases/{release_id}", false},
		{"curl -X GET https://api.github.com/repos/o/r/releases/1", false},
		{"Antes, chame `GET /repos/{owner}/{repo}/releases` para pegar o id.", true},
		{"use /repos/{owner}/{repo}/releases para listar", true},
		{"GET /repos/{owner}/{repo}/releases", true},
	}
	for _, c := range cases {
		if got := mentionsCall(c.answer, list); got != c.want {
			t.Errorf("%q: %v, esperava %v", c.answer, got, c.want)
		}
	}
}
