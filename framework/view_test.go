package framework

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type endpointViewTest struct {
	Method string
	Path   string
}

func TestViewEngineRendersBladeLikeLayout(t *testing.T) {
	dir := t.TempDir()
	writeView(t, dir, "layouts/app.ordin.html", `<!doctype html>
<title>@yield("title")</title>
<body>
@include("partials.nav")
@yield("content")
</body>`)
	writeView(t, dir, "partials/nav.ordin.html", `<nav>{{ brand }}</nav>`)
	writeView(t, dir, "welcome.ordin.html", `@extends("layouts.app")
@section("title"){{ title }}@endsection
@section("content")
@if show
<ul>
@foreach endpoints as endpoint
<li>{{ endpoint.Method }} {{ endpoint.Path }}</li>
@endforeach
</ul>
@else
<p>hidden</p>
@endif
@endsection`)

	engine := MustViewEngine(dir)
	recorder := httptest.NewRecorder()

	err := engine.Render(recorder, http.StatusOK, "welcome", Data{
		"brand": "ORDIN",
		"title": "Home",
		"show":  true,
		"endpoints": []endpointViewTest{
			{Method: "GET", Path: "/api/users"},
		},
	})
	if err != nil {
		t.Fatalf("render view: %v", err)
	}

	body := recorder.Body.String()
	for _, expected := range []string{"<title>Home</title>", "<nav>ORDIN</nav>", "<li>GET /api/users</li>"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected body to contain %q, got:\n%s", expected, body)
		}
	}
}

func TestViewEngineEscapesByDefaultAndSupportsRaw(t *testing.T) {
	dir := t.TempDir()
	writeView(t, dir, "welcome.ordin.html", `<p>{{ value }}</p><div>{!! html !!}</div>`)

	engine := MustViewEngine(dir)
	recorder := httptest.NewRecorder()

	err := engine.Render(recorder, http.StatusOK, "welcome", Data{
		"value": `<script>alert("x")</script>`,
		"html":  `<strong>safe by caller</strong>`,
	})
	if err != nil {
		t.Fatalf("render view: %v", err)
	}

	body := recorder.Body.String()
	if strings.Contains(body, `<script>alert("x")</script>`) {
		t.Fatalf("escaped output rendered raw script: %s", body)
	}
	if !strings.Contains(body, `<strong>safe by caller</strong>`) {
		t.Fatalf("raw output missing: %s", body)
	}
}

func writeView(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir view dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write view: %v", err)
	}
}
