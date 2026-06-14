package nobelprize

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// These tests are offline: they exercise the URI driver's pure string functions
// and the host wiring. The client's HTTP behaviour is covered in nobelprize_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "nobelprize" {
		t.Errorf("Scheme = %q, want nobelprize", info.Scheme)
	}
	if info.Identity.Binary != "nobelprize" {
		t.Errorf("Identity.Binary = %q, want nobelprize", info.Identity.Binary)
	}
	if info.Identity.Repo == "" {
		t.Error("Identity.Repo is empty")
	}
}

func TestClassify(t *testing.T) {
	d := Domain{}

	cases := []struct {
		in      string
		wantTyp string
		wantID  string
	}{
		{"2023", "year", "2023"},
		{"1921", "year", "1921"},
		{"physics", "category", "physics"},
		{"medicine", "category", "medicine"},
		{"Peace", "category", "peace"},
		{"einstein", "laureate", "einstein"},
		{"Marie Curie", "laureate", "Marie Curie"},
		{"https://www.nobelprize.org/search/?q=einstein", "laureate", "search"},
	}

	for _, tc := range cases {
		typ, id, err := d.Classify(tc.in)
		if err != nil {
			t.Errorf("Classify(%q) error = %v", tc.in, err)
			continue
		}
		if typ != tc.wantTyp || id != tc.wantID {
			t.Errorf("Classify(%q) = (%q, %q), want (%q, %q)", tc.in, typ, id, tc.wantTyp, tc.wantID)
		}
	}

	_, _, err := d.Classify("")
	if err == nil {
		t.Error("Classify('') = nil error, want error")
	}
}

func TestLocate(t *testing.T) {
	d := Domain{}

	url, err := d.Locate("laureate", "einstein")
	if err != nil {
		t.Fatalf("Locate(laureate, einstein) error = %v", err)
	}
	if url == "" {
		t.Error("Locate(laureate, einstein) returned empty URL")
	}

	url, err = d.Locate("year", "2023")
	if err != nil {
		t.Fatalf("Locate(year, 2023) error = %v", err)
	}
	want := "https://www.nobelprize.org/prizes/2023"
	if url != want {
		t.Errorf("Locate(year, 2023) = %q, want %q", url, want)
	}

	url, err = d.Locate("category", "physics")
	if err != nil {
		t.Fatalf("Locate(category, physics) error = %v", err)
	}
	if url == "" {
		t.Error("Locate(category, physics) returned empty URL")
	}

	_, err = d.Locate("unknown", "foo")
	if err == nil {
		t.Error("Locate(unknown, foo) = nil error, want error")
	}
}

func TestDomainRegistered(t *testing.T) {
	// init() registered the domain; kit.Open should find it.
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Domain("nobelprize"); !ok {
		t.Fatal("nobelprize domain not registered")
	}
}

func TestResolveCategoryMapping(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"physics", "phy"},
		{"chemistry", "che"},
		{"medicine", "med"},
		{"literature", "lit"},
		{"peace", "pea"},
		{"economics", "eco"},
		{"phy", "phy"},
		{"", ""},
	}
	for _, tc := range cases {
		got := resolveCategory(tc.in)
		if got != tc.want {
			t.Errorf("resolveCategory(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFirstLang(t *testing.T) {
	cases := []struct {
		m    wireMultiLang
		want string
	}{
		{wireMultiLang{En: "English", Se: "Swedish", No: "Norwegian"}, "English"},
		{wireMultiLang{En: "", Se: "Swedish", No: "Norwegian"}, "Swedish"},
		{wireMultiLang{En: "", Se: "", No: "Norwegian"}, "Norwegian"},
		{wireMultiLang{}, ""},
	}
	for _, tc := range cases {
		got := firstLang(tc.m)
		if got != tc.want {
			t.Errorf("firstLang(%+v) = %q, want %q", tc.m, got, tc.want)
		}
	}
}
