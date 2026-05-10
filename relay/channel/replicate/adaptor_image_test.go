package replicate

import "testing"

func TestMapOpenAISizeToFluxSupportsThreeToOneRatios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		size       string
		wantAspect string
	}{
		{size: "3072x1024", wantAspect: "3:1"},
		{size: "1280x3840", wantAspect: "1:3"},
	}

	for _, tt := range tests {
		t.Run(tt.size, func(t *testing.T) {
			aspect, _, _, ok := mapOpenAISizeToFlux(tt.size)
			if !ok {
				t.Fatalf("mapOpenAISizeToFlux(%q) returned ok=false", tt.size)
			}
			if aspect != tt.wantAspect {
				t.Fatalf("mapOpenAISizeToFlux(%q) aspect = %q, want %q", tt.size, aspect, tt.wantAspect)
			}
		})
	}
}
