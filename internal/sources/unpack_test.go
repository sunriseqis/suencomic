package sources

import (
	"strings"
	"testing"
)

func TestUnpackDeanEdwards(t *testing.T) {
	// p(a,c,k,e,d) sample: payload "0 1", radix 36, 2 words, symtab alpha|beta
	packed := `eval(function(p,a,c,k,e,d){while(--c) e[c]=c; e=e.join('')}('0 1',36,2,'alpha|beta'.split('|'),0,{}))`
	got := UnpackDeanEdwards(packed)
	if !strings.Contains(got, "alpha") || !strings.Contains(got, "beta") {
		t.Fatalf("expected unpacked to contain alpha/beta, got %q", got)
	}
	if strings.Contains(got, "'0 1'") {
		t.Fatalf("expected packed payload replaced, got %q", got)
	}
}

func TestUnpackDeanEdwardsLeavesPlainInput(t *testing.T) {
	plain := "var x = 1;"
	if got := UnpackDeanEdwards(plain); got != plain {
		t.Fatalf("expected unchanged passthrough, got %q", got)
	}
}

func TestExtractImageURLsFromJSFullURLs(t *testing.T) {
	js := `var dirs=["/1_1743.jpg","/2_6582.jpg"];var pix="https://image.mangabz.com/1/139/418076";`
	// full URLs present path
	js2 := `var x=["https://img.example.com/a.jpg","https://img.example.com/b.webp"];`
	urls := ExtractImageURLsFromJS(js2)
	if len(urls) != 2 {
		t.Fatalf("expected 2 urls, got %v", urls)
	}
	if urls[0] != "https://img.example.com/a.jpg" || urls[1] != "https://img.example.com/b.webp" {
		t.Fatalf("unexpected urls: %v", urls)
	}
	_ = js
}

func TestExtractImageURLsFromJSPixPrefix(t *testing.T) {
	js := `var pix="https://image.mangabz.com/1/139/418076/";var filenames=["1_1.jpg","2_2.jpg"];`
	urls := ExtractImageURLsFromJS(js)
	if len(urls) != 2 {
		t.Fatalf("expected 2 urls, got %v", urls)
	}
	for _, u := range urls {
		if !strings.HasPrefix(u, "https://image.mangabz.com/") {
			t.Fatalf("expected pix-prefixed url, got %q", u)
		}
	}
}
