package views

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/G2HJei/job-search-tracker/internal/model"
	"github.com/G2HJei/job-search-tracker/internal/views/components"
)

func render(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestComponents(t *testing.T) {
	cfg := model.DefaultConfig()
	tests := []struct {
		name string
		c    templ.Component
		want []string
		not  []string
	}{
		{"known status", components.StatusPill(&cfg, "interviewing"), []string{`class="pill status-interviewing"`, "Interviewing"}, nil},
		{"closed status", components.StatusPill(&cfg, "ghosted"), []string{"closed", "No response"}, nil},
		{"unknown status", components.StatusPill(&cfg, "on-hold"), []string{`class="pill unknown"`, "on-hold", "Not in config.yaml"}, nil},
		{"priority color", components.PriorityPill(&cfg, "high"), []string{"color-red", "High"}, nil},
		{"rating", components.Rating(3), []string{`★★★<span class="rating-empty">☆☆</span>`, "3 of 5"}, nil},
		{"rating full", components.Rating(5), []string{"★★★★★"}, []string{"☆"}},
		{"rating unset", components.Rating(0), []string{"–"}, []string{"★"}},
		{"unknown option kept", components.SelectOnly("channel", "Meetup", components.Strings(cfg.Channels), "–", nil),
			[]string{`<option value="Meetup" selected>Meetup (not in config)</option>`, `<option value="">–</option>`}, nil},
		{"markdown", components.Markdown("**hi** <script>x</script> [a](javascript:alert(1)) https://example.com"),
			[]string{"<strong>hi</strong>", `href="https://example.com"`}, []string{"<script>", "javascript:"}},
		{"markdown empty", components.Markdown("   "), nil, []string{"md"}},
		{"textarea keeps value exact", components.TextArea("Notes", "notes", "- a\n- b\n", "", 2),
			[]string{"rows=\"2\">- a\n- b\n</textarea>"}, nil},
		{"input error", components.Input("Title", "text", "title", "x", "Required", nil),
			[]string{`aria-invalid="true"`, `<small class="error">Required</small>`}, nil},
		{"escaping", components.ContactView(model.Contact{Name: `<b>Eve</b>`, LinkedIn: "javascript:alert(1)"}),
			[]string{"&lt;b&gt;Eve&lt;/b&gt;", "about:invalid"}, []string{"<b>Eve", `href="javascript:`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := render(t, tt.c)
			for _, w := range tt.want {
				if !strings.Contains(out, w) {
					t.Errorf("missing %q in\n%s", w, out)
				}
			}
			for _, n := range tt.not {
				if strings.Contains(out, n) {
					t.Errorf("unexpected %q in\n%s", n, out)
				}
			}
		})
	}
}

func TestDashboardEmptyState(t *testing.T) {
	cfg := model.DefaultConfig()
	out := render(t, DashboardPage(Page{Title: "Dashboard"}, DashboardData{Config: cfg}))
	if !strings.Contains(out, "No applications yet") {
		t.Fatalf("no empty state:\n%s", out)
	}
}

func TestLoadErrorBanner(t *testing.T) {
	out := render(t, Layout(Page{Title: "x", LoadErrors: 2}))
	if !strings.Contains(out, "2 files could not be loaded") || !strings.Contains(out, "Reload from disk") {
		t.Fatalf("banner missing:\n%s", out)
	}
	if strings.Contains(render(t, Layout(Page{Title: "x"})), "could not be loaded") {
		t.Fatal("banner shown without errors")
	}
}
