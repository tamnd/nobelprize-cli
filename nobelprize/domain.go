package nobelprize

import (
	"context"
	"net/url"
	"regexp"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go registers the nobelprize kit Domain so a blank import in a
// multi-domain host (ant) enables the driver:
//
//	import _ "github.com/tamnd/nobelprize-cli/nobelprize"
//
// The Domain also builds the standalone nobelprize binary via NewApp.
func init() { kit.Register(Domain{}) }

// Domain is the Nobel Prize driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme and the identity the single-site binary inherits.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "nobelprize",
		Hosts:  []string{Host, "www.nobelprize.org", "nobelprize.org"},
		Identity: kit.Identity{
			Binary: "nobelprize",
			Short:  "Explore Nobel Prize data",
			Long: `Explore Nobel Prize data via the official Nobel Prize API.

nobelprize reads public Nobel Prize data over plain HTTPS, shapes it into
clean records, and prints output that pipes into the rest of your tools. No API
key, nothing to run alongside it.`,
			Site: "www.nobelprize.org",
			Repo: "https://github.com/tamnd/nobelprize-cli",
		},
	}
}

// Register installs the client factory and the two Nobel Prize operations onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:    "prizes",
		Group:   "browse",
		Summary: "List Nobel prizes",
		Args:    []kit.Arg{{Name: "category", Help: "category filter (optional)", Optional: true}},
	}, listPrizes)

	kit.Handle(app, kit.OpMeta{
		Name:    "laureates",
		Group:   "browse",
		Summary: "Search Nobel laureates",
		Args:    []kit.Arg{{Name: "name", Help: "search by name (optional)", Optional: true}},
	}, listLaureates)
}

// newClient builds a Client from the resolved kit Config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	return NewClient(c), nil
}

// --- input structs ---

type prizesInput struct {
	Category string  `kit:"flag" help:"category: physics, chemistry, medicine, literature, peace, economics"`
	Year     string  `kit:"flag" help:"award year (e.g., 2023)"`
	Limit    int     `kit:"flag,inherit" help:"max results"`
	Client   *Client `kit:"inject"`
}

type laureatesInput struct {
	Name     string  `kit:"arg" help:"search by name (optional)"`
	Category string  `kit:"flag" help:"category filter"`
	Limit    int     `kit:"flag,inherit" help:"max results"`
	Client   *Client `kit:"inject"`
}

// --- handlers ---

func listPrizes(ctx context.Context, in prizesInput, emit func(Prize) error) error {
	prizes, err := in.Client.ListPrizes(ctx, in.Category, in.Year, in.Limit)
	if err != nil {
		return err
	}
	for _, p := range prizes {
		if err := emit(p); err != nil {
			return err
		}
	}
	return nil
}

func listLaureates(ctx context.Context, in laureatesInput, emit func(Laureate) error) error {
	laureates, err := in.Client.SearchLaureates(ctx, in.Name, in.Category, in.Limit)
	if err != nil {
		return err
	}
	for _, l := range laureates {
		if err := emit(l); err != nil {
			return err
		}
	}
	return nil
}

// --- Resolver ---

// yearRE matches a 4-digit year.
var yearRE = regexp.MustCompile(`^\d{4}$`)

// knownCategoryNames is the set of human-friendly category names.
var knownCategoryNames = map[string]bool{
	"physics":    true,
	"chemistry":  true,
	"medicine":   true,
	"literature": true,
	"peace":      true,
	"economics":  true,
}

// Classify turns any accepted input into the canonical (uriType, id).
// 4-digit year → ("year", year); known category → ("category", name);
// anything else → ("laureate", input).
func (Domain) Classify(input string) (uriType, id string, err error) {
	if input == "" {
		return "", "", errs.Usage("nobelprize: empty input")
	}
	// strip URL if it's a full URL
	if u, err2 := url.Parse(input); err2 == nil && (u.Scheme == "http" || u.Scheme == "https") {
		input = strings.Trim(u.Path, "/")
	}
	input = strings.TrimSpace(input)
	if yearRE.MatchString(input) {
		return "year", input, nil
	}
	lower := strings.ToLower(input)
	if knownCategoryNames[lower] {
		return "category", lower, nil
	}
	return "laureate", input, nil
}

// Locate returns the canonical Nobel Prize URL for a (uriType, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "laureate":
		return "https://www.nobelprize.org/search/?q=" + url.QueryEscape(id), nil
	case "year":
		return "https://www.nobelprize.org/prizes/" + id, nil
	case "category":
		return "https://www.nobelprize.org/prizes/" + id + "/", nil
	default:
		return "", errs.Usage("nobelprize has no resource type %q", uriType)
	}
}

// mapErr converts a library error into the kit error kind with the right exit code.
func mapErr(err error) error {
	return err
}
