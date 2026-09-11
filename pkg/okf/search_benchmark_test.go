package okf_test

import (
	"fmt"
	"testing"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
)

func BenchmarkSearchUnicode(b *testing.B) {
	concepts := make(map[string]*okf.Concept, 100)
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("research/concept-%03d", i)
		concepts[id] = &okf.Concept{
			ID:          id,
			Title:       fmt.Sprintf("Тензорная модель %d", i),
			Description: "Sparse reconstruction and dynamic systems analysis",
			Body:        "Low-rank tensor decomposition for scientific machine learning.",
		}
	}
	bundle := &okf.Bundle{Concepts: concepts}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bundle.Search("тензорная reconstruction", 5)
	}
}
