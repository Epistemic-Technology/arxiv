package arxivgo

import (
	"testing"
)

func TestMakeGetQuery(t *testing.T) {
	tests := []struct {
		params SearchParams
		want   string
	}{
		{
			SearchParams{
				Query: "all:electron",
			},
			"search_query=all%3Aelectron",
		},
		{
			SearchParams{
				Query:  "all:galaxy",
				IdList: []string{"2408.03982", "2408.03988"},
			},
			"id_list=2408.03982%2C2408.03988&search_query=all%3Agalaxy",
		},
		{
			SearchParams{
				Query:      "all:galaxy",
				IdList:     []string{"2408.03982", "2408.03988"},
				Start:      10,
				MaxResults: 5,
			},
			"id_list=2408.03982%2C2408.03988&max_results=5&search_query=all%3Agalaxy&start=10",
		},
		{
			SearchParams{
				Query: "au:del_maestro ANDNOT (ti:checkerboard OR ti:Pyrochlore)",
			},
			"search_query=au%3Adel_maestro+ANDNOT+%28ti%3Acheckerboard+OR+ti%3APyrochlore%29",
		},
	}
	for _, test := range tests {
		got := makeGetQuery(test.params)
		if got != test.want {
			t.Errorf("makeGetQ(%v) = %v; want %v", test.params, got, test.want)
		}
	}
}

func TestGetRequestGetsOKResponseWithDefaultConfig(t *testing.T) {
	params := SearchParams{
		Query: "all:electron",
	}
	resp, err := doGetRequest(DefaultConfig, params)
	if err != nil {
		t.Errorf("GetRequest(%v) = %v; want nil", params, err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("GetRequest(%v) = %v; want 200", params, resp.StatusCode)
	}
}

func TestSearchWorksWithDefaultConfig(t *testing.T) {
	requester := MakeRequester(DefaultConfig)
	params := SearchParams{
		Query: "all:electron",
	}
	_, err := Search(requester, params)
	if err != nil {
		t.Errorf("Search(%v) = %v; want nil", params, err)
	}
}
