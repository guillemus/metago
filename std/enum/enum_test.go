package enum

import "testing"

func TestArticleStatus(t *testing.T) {
	if got, want := ArticleStatusPublished.String(), "published"; got != want {
		t.Errorf("ArticleStatusPublished.String() = %q, want %q", got, want)
	}
}
