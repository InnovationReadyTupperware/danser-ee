package buffer

import "testing"

func TestValidateRequestedMultisampleSamples(t *testing.T) {
	tests := []struct {
		name       string
		samples    int
		maxSamples int
		wantErr    bool
	}{
		{name: "disabled", samples: 0, maxSamples: 0},
		{name: "supported", samples: 16, maxSamples: 16},
		{name: "below limit", samples: 8, maxSamples: 16},
		{name: "above limit", samples: 16, maxSamples: 8, wantErr: true},
		{name: "negative", samples: -1, maxSamples: 16, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRequestedMultisampleSamples(tt.samples, tt.maxSamples)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateRequestedMultisampleSamples(%d, %d) error = %v, wantErr %t", tt.samples, tt.maxSamples, err, tt.wantErr)
			}
		})
	}
}
