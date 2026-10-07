package main

import "testing"

func TestMergeSkipsDuplicatesAndOrphans(t *testing.T) {
	existing := &Store{
		Projects: []Project{{ID: "p1", Name: "one"}},
		Tunnels:  []TunnelConfig{{ID: "t1", ProjectID: "p1", ServiceName: "a"}},
	}
	incoming := &Store{
		Projects: []Project{
			{ID: "p1", Name: "one"}, // duplicate → skipped
			{ID: "p2", Name: "two"}, // new
		},
		Tunnels: []TunnelConfig{
			{ID: "t1", ProjectID: "p1"}, // duplicate → skipped
			{ID: "t2", ProjectID: "p2"}, // new
			{ID: "t3", ProjectID: "p9"}, // orphan (no such project) → skipped
		},
	}

	projects, tunnels := existing.Merge(incoming)
	if projects != 1 || tunnels != 1 {
		t.Fatalf("got %d projects / %d tunnels added, want 1/1", projects, tunnels)
	}
	if len(existing.Projects) != 2 || len(existing.Tunnels) != 2 {
		t.Fatalf("store ended with %d projects / %d tunnels", len(existing.Projects), len(existing.Tunnels))
	}
}

func TestIsNewer(t *testing.T) {
	cases := []struct {
		candidate, current string
		want               bool
	}{
		{"2026.09.20", "2026.09.17", true},
		{"2026.09.17", "2026.09.17", false},
		{"2026.09.02", "2026.09.17", false},
		{"1.2.0", "1.10.0", false},
		{"1.10.0", "1.2.0", true},
		{"2027.01.01", "2026.12.31", true},
	}
	for _, c := range cases {
		if got := isNewer(c.candidate, c.current); got != c.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", c.candidate, c.current, got, c.want)
		}
	}
}

func TestMergeRenamesClashingProjectNames(t *testing.T) {
	existing := &Store{Projects: []Project{{ID: "p1", Name: "EEC"}}}
	incoming := &Store{Projects: []Project{{ID: "p2", Name: "EEC"}}}

	existing.Merge(incoming)
	if got := existing.Projects[1].Name; got != "EEC (2)" {
		t.Fatalf("imported project named %q, want %q", got, "EEC (2)")
	}
}

func TestSubsetKeepsOnlyChosenProjects(t *testing.T) {
	s := &Store{
		Projects: []Project{{ID: "p1"}, {ID: "p2"}, {ID: "p3"}},
		Tunnels: []TunnelConfig{
			{ID: "t1", ProjectID: "p1"},
			{ID: "t2", ProjectID: "p2"},
			{ID: "t3", ProjectID: "p3"},
			{ID: "t4", ProjectID: "p3"},
		},
	}
	sub := s.Subset([]string{"p3", "p1"})
	if len(sub.Projects) != 2 || sub.Projects[0].ID != "p1" || sub.Projects[1].ID != "p3" {
		t.Fatalf("got projects %+v", sub.Projects)
	}
	if len(sub.Tunnels) != 3 {
		t.Fatalf("got %d tunnels, want 3", len(sub.Tunnels))
	}
}

func TestFileSlug(t *testing.T) {
	cases := map[string]string{
		"EEC Prod":    "eec-prod",
		"  a/b\\c  ":  "a-b-c",
		"ระบบ UAT #2": "ระบบ-uat-2",
		"!!!":         "project",
	}
	for in, want := range cases {
		if got := fileSlug(in); got != want {
			t.Errorf("fileSlug(%q) = %q, want %q", in, got, want)
		}
	}
}
